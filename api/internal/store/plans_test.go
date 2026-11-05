package store_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Aegisystem/Quitsie/api/internal/store"
	"github.com/Aegisystem/Quitsie/api/internal/storetest"
)

// Sign-in deletes the expired rows of these tables inside its write, while
// other writes wait (store.BeginWrite). A scan of a large table there makes
// every other write wait longer.
func TestTheDeletesOfExpiredRowsUseAnIndex(t *testing.T) {
	ctx := context.Background()
	conn, err := storetest.Open(t).Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	// The tables are empty, and for an empty table a scan is the cheapest
	// plan. Making scans expensive shows whether an index could serve.
	if _, err := conn.ExecContext(ctx, `SET enable_seqscan = off`); err != nil {
		t.Fatal(err)
	}

	for query, index := range map[string]string{
		store.DeleteExpiredLoginCodes:  "login_codes_expires_at",
		store.DeleteExpiredSessions:    "sessions_last_used_at",
		store.DeleteEndedLoginFailures: "login_failures_resets_at",
		store.DeleteStaleKnownDevices:  "known_devices_last_used_at",
	} {
		name := strings.Fields(query)[2]
		rows, err := conn.QueryContext(ctx, "EXPLAIN "+query, time.Now())
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var plan []string
		for rows.Next() {
			var line string
			if err := rows.Scan(&line); err != nil {
				t.Fatal(err)
			}
			plan = append(plan, line)
		}
		if err := rows.Close(); err != nil {
			t.Fatal(err)
		}
		all := strings.Join(plan, "\n")
		if strings.Contains(all, "Seq Scan") || !strings.Contains(all, " on "+index) {
			t.Errorf("%s: plan =\n%s\nwant a scan of %s", name, all, index)
		}
	}
}
