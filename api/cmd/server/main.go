// Command server runs Quitsie from a single binary: the API under /api/v1,
// the embedded PWA, and the SQLite database it migrates at startup.
//
// Configuration comes from the environment:
//
//	QUITSIE_ADDR     address to listen on (default 127.0.0.1:8080; cloudflared connects to it)
//	QUITSIE_DB_PATH  SQLite file (default quitsie.db)
package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Aegisystem/Quitsie/api/internal/httpapi"
	"github.com/Aegisystem/Quitsie/api/internal/store"
)

// dist holds the PWA that `npm run build` writes from web/.
//
//go:embed all:dist
var dist embed.FS

// version is set at build time with -ldflags "-X main.version=<commit>".
var version = "dev"

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(logger); err != nil {
		logger.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	addr := envOr("QUITSIE_ADDR", "127.0.0.1:8080")
	dbPath := envOr("QUITSIE_DB_PATH", "quitsie.db")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := store.Open(ctx, dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := store.Migrate(ctx, db); err != nil {
		return err
	}

	pwa, err := fs.Sub(dist, "dist")
	if err != nil {
		return fmt.Errorf("embedded PWA: %w", err)
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           httpapi.NewHandler(httpapi.NewServer(db, version).WithDeps(httpapi.Deps{DB: db}), pwa),
		ReadHeaderTimeout: 10 * time.Second,
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.ListenAndServe() }()
	logger.Info("listening", "url", "http://"+addr, "version", version, "db", dbPath)

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
	}

	// systemd sends SIGTERM on restart: finish in-flight requests, then close the database.
	logger.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
