package store_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/Aegisystem/Quitsie/api/internal/store"
)

// handOut spends n versions, as n writes would.
func handOut(t *testing.T, db *sql.DB, n int) int64 {
	t.Helper()
	var version int64
	for range n {
		version = write(t, db, func(*store.Queries, int64) error { return nil })
	}
	return version
}

func recordRestore(t *testing.T, db *sql.DB, at time.Time) store.RestoreGap {
	t.Helper()
	gap, err := store.RecordRestore(context.Background(), db, at)
	if err != nil {
		t.Fatal(err)
	}
	return gap
}

func inGap(t *testing.T, db *sql.DB, cursor int64) bool {
	t.Helper()
	got, err := store.New(db).InRestoreGap(context.Background(), cursor)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestARestoreJumpsTheCounterToTheClock(t *testing.T) {
	db := openMigrated(t)
	restored := handOut(t, db, 3)

	gap := recordRestore(t, db, now)

	want := store.RestoreGap{RestoredVersion: restored, JumpedVersion: now.UnixMicro(), RestoredAt: now}
	if gap != want {
		t.Errorf("gap = %+v, want %+v", gap, want)
	}
	if next := handOut(t, db, 1); next != now.UnixMicro()+1 {
		t.Errorf("the next write got version %d, want %d", next, now.UnixMicro()+1)
	}
	var stored store.RestoreGap
	err := db.QueryRow(`SELECT restored_version, jumped_version, restored_at FROM restore_gaps`).
		Scan(&stored.RestoredVersion, &stored.JumpedVersion, &stored.RestoredAt)
	if err != nil {
		t.Fatal(err)
	}
	if !stored.RestoredAt.Equal(now) || stored.RestoredVersion != restored || stored.JumpedVersion != now.UnixMicro() {
		t.Errorf("stored gap = %+v, want %+v", stored, want)
	}
}

func TestARestoreJumpsAtLeastABillionWhenTheClockIsLate(t *testing.T) {
	db := openMigrated(t)
	restored := handOut(t, db, 3)

	gap := recordRestore(t, db, time.Unix(0, 0))

	if gap.JumpedVersion != restored+1_000_000_000 {
		t.Errorf("jumped to %d, want %d", gap.JumpedVersion, restored+1_000_000_000)
	}
}

func TestARestoreRefusesAClockThatWouldOutgrowThePWA(t *testing.T) {
	db := openMigrated(t)
	restored := handOut(t, db, 3)

	_, err := store.RecordRestore(context.Background(), db, time.Date(2300, 1, 1, 0, 0, 0, 0, time.UTC))

	if err == nil {
		t.Fatal("RecordRestore accepted a jump past 2^53")
	}
	if got, _ := store.New(db).CurrentVersion(context.Background()); got != restored {
		t.Errorf("version = %d after a refused restore, want it untouched at %d", got, restored)
	}
	if inGap(t, db, restored+1) {
		t.Error("a refused restore left a gap")
	}
}

func TestOnlyCursorsStrictlyInsideAGapAreInIt(t *testing.T) {
	db := openMigrated(t)
	handOut(t, db, 5)
	first := recordRestore(t, db, now)
	handOut(t, db, 5)
	second := recordRestore(t, db, now.Add(time.Hour))

	for _, c := range []struct {
		cursor int64
		want   bool
	}{
		{0, false},
		{first.RestoredVersion, false},
		{first.RestoredVersion + 1, true},
		{first.JumpedVersion - 1, true},
		{first.JumpedVersion, false},
		{second.RestoredVersion, false},
		{second.RestoredVersion + 1, true},
		{second.JumpedVersion - 1, true},
		{second.JumpedVersion, false},
	} {
		if got := inGap(t, db, c.cursor); got != c.want {
			t.Errorf("InRestoreGap(%d) = %v, want %v (gaps %+v and %+v)", c.cursor, got, c.want, first, second)
		}
	}
}

func TestRecordingTheSameRestoreTwiceStrandsNoCursor(t *testing.T) {
	db := openMigrated(t)
	handOut(t, db, 5)
	first := recordRestore(t, db, now)

	again := recordRestore(t, db, now)

	if again.RestoredVersion != first.JumpedVersion {
		t.Errorf("the second record starts at %d, want the first jump, %d", again.RestoredVersion, first.JumpedVersion)
	}
	if inGap(t, db, first.JumpedVersion) {
		t.Error("the cursor the first jump handed out falls in the second gap")
	}
}
