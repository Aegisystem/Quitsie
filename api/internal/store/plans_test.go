package store

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Sign-in deletes the expired rows of these tables inside its write, with the
// write lock held. A scan of a large table there makes every other write wait.
func TestTheDeletesOfExpiredRowsUseAnIndex(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "quitsie.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}

	for query, index := range map[string]string{
		deleteExpiredLoginCodes:  "login_codes_expires_at",
		deleteExpiredSessions:    "sessions_last_used_at",
		deleteEndedLoginFailures: "login_failures_resets_at",
		deleteStaleKnownDevices:  "known_devices_last_used_at",
	} {
		name := strings.Fields(query)[2]
		rows, err := db.QueryContext(ctx, "EXPLAIN QUERY PLAN "+query, time.Now())
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var plan []string
		for rows.Next() {
			var id, parent, unused int
			var detail string
			if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
				t.Fatal(err)
			}
			plan = append(plan, detail)
		}
		if err := rows.Close(); err != nil {
			t.Fatal(err)
		}
		if len(plan) != 1 || !strings.HasPrefix(plan[0], "SEARCH ") || !strings.Contains(plan[0], " INDEX "+index+" ") {
			t.Errorf("%s: plan = %q, want a search with %s", name, plan, index)
		}
	}
}
