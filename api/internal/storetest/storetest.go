// Package storetest gives each test its own migrated PostgreSQL database, so
// tests stay isolated and can run in parallel.
//
// It needs a PostgreSQL server where it may create databases: the one in
// compose.yaml (docker compose up -d), or the one QUITSIE_TEST_DATABASE_URL
// names.
package storetest

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"sync"
	"testing"

	"github.com/Aegisystem/Quitsie/api/internal/store"
	"github.com/Aegisystem/Quitsie/api/migrations"
)

// defaultURL is the PostgreSQL of compose.yaml.
const defaultURL = "postgres://quitsie:quitsie@127.0.0.1:5433/quitsie?sslmode=disable"

var (
	adminOnce sync.Once
	admin     *sql.DB
	template  string
	adminErr  error

	// names holds the database name of each *sql.DB this package opened.
	names sync.Map
)

// Open returns a new, empty, migrated database that is dropped when t ends.
func Open(t testing.TB) *sql.DB {
	t.Helper()
	return create(t, templateName(t))
}

// Copy returns a new database with what db holds now, as a backup keeps it.
// db goes on working, and later writes to it do not reach the copy. db must
// come from Open or Copy, and nothing may be using it during the call.
func Copy(t testing.TB, db *sql.DB) *sql.DB {
	t.Helper()
	source, ok := names.Load(db)
	if !ok {
		t.Fatal("storetest: Copy needs a database from Open or Copy")
	}
	// PostgreSQL copies a database only while nobody is connected to it.
	db.SetMaxIdleConns(0)
	defer db.SetMaxIdleConns(10)
	return create(t, source.(string))
}

// URL is the server the tests use, pointing at its maintenance database.
func URL() string {
	if u := os.Getenv("QUITSIE_TEST_DATABASE_URL"); u != "" {
		return u
	}
	return defaultURL
}

// URLFor is URL pointing at the database name instead.
func URLFor(t testing.TB, name string) string {
	t.Helper()
	u, err := url.Parse(URL())
	if err != nil {
		t.Fatalf("storetest: QUITSIE_TEST_DATABASE_URL: %v", err)
	}
	u.Path = "/" + name
	return u.String()
}

// URLOf is the URL of db, which must come from Open or Copy, for code that
// opens the database on its own.
func URLOf(t testing.TB, db *sql.DB) string {
	t.Helper()
	name, ok := names.Load(db)
	if !ok {
		t.Fatal("storetest: URLOf needs a database from Open or Copy")
	}
	return URLFor(t, name.(string))
}

// create makes a database from the template from and opens it.
func create(t testing.TB, from string) *sql.DB {
	t.Helper()
	ctx := context.Background()
	name := "quitsie_test_" + random()
	if _, err := admin.ExecContext(ctx, fmt.Sprintf(`CREATE DATABASE %s TEMPLATE %s`, name, from)); err != nil {
		t.Fatalf("storetest: create database: %v", err)
	}
	db, err := store.Open(ctx, URLFor(t, name))
	if err != nil {
		t.Fatal(err)
	}
	names.Store(db, name)
	t.Cleanup(func() {
		db.Close()
		names.Delete(db)
		if _, err := admin.ExecContext(context.Background(), fmt.Sprintf(`DROP DATABASE %s WITH (FORCE)`, name)); err != nil {
			t.Errorf("storetest: drop database: %v", err)
		}
	})
	return db
}

// templateName is a migrated database that every test copies, so each one
// pays for a copy instead of for the migrations. Its name comes from the
// migrations, so a change to them makes a new one; test binaries running at
// once share it.
func templateName(t testing.TB) string {
	t.Helper()
	adminOnce.Do(func() { adminErr = prepare() })
	if adminErr != nil {
		t.Fatalf("storetest: %v (is PostgreSQL running? docker compose up -d)", adminErr)
	}
	return template
}

func prepare() error {
	ctx := context.Background()
	var err error
	if admin, err = store.Open(ctx, URL()); err != nil {
		return err
	}
	admin.SetMaxOpenConns(4)
	if template, err = migrationsHash(); err != nil {
		return err
	}
	template = "quitsie_template_" + template

	// One test binary at a time creates the template; the others wait here.
	conn, err := admin.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, `SELECT pg_advisory_lock(42)`); err != nil {
		return err
	}
	defer conn.ExecContext(ctx, `SELECT pg_advisory_unlock(42)`)

	var exists bool
	if err := conn.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)`, template).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}
	building := template + "_building"
	if _, err := conn.ExecContext(ctx, fmt.Sprintf(`DROP DATABASE IF EXISTS %s WITH (FORCE)`, building)); err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, fmt.Sprintf(`CREATE DATABASE %s`, building)); err != nil {
		return err
	}
	u, _ := url.Parse(URL())
	u.Path = "/" + building
	db, err := store.Open(ctx, u.String())
	if err != nil {
		return err
	}
	err = store.Migrate(ctx, db)
	db.Close()
	if err != nil {
		return err
	}
	// Renamed only once complete, so a binary that fails halfway leaves no
	// template the others would trust.
	_, err = conn.ExecContext(ctx, fmt.Sprintf(`ALTER DATABASE %s RENAME TO %s`, building, template))
	return err
}

// migrationsHash identifies the schema the migrations build.
func migrationsHash() (string, error) {
	h := sha256.New()
	err := fs.WalkDir(migrations.FS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(migrations.FS, path)
		h.Write([]byte(path))
		h.Write(b)
		return err
	})
	return hex.EncodeToString(h.Sum(nil))[:16], err
}

func random() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}
