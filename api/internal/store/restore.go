package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// A restored backup is an older copy of the database. The copy that was lost
// may have handed out versions above the backup's counter, and devices may
// hold cursors among them. If the counter carried on from the backup, it would
// hand those versions out again, and a device whose cursor it passed would
// skip the new rows and keep the lost ones forever. So the counter jumps past
// anything the lost copy could have reached, and GET /sync refuses the cursors
// in between (docs/arquitectura.md, "Restaurar un respaldo").

// minRestoreJump is the least a restore moves the counter: more versions than
// the lost copy can have handed out after the backup. A backup of the hosted
// database may be a day or more old; a billion versions is 11 days at a
// thousand writes a second, far more than this server sustains.
const minRestoreJump = 1_000_000_000

// maxVersion is the largest integer a JavaScript number holds exactly, and the
// PWA reads versions and cursors as numbers.
const maxVersion = 1<<53 - 1

// RecordRestore jumps version_counter after a restore and records the versions
// it skipped, in one transaction. Call it before the server hands out any
// version.
//
// The counter jumps to the clock in microseconds, or by minRestoreJump if that
// goes further, which covers a clock that runs late and two restores minutes
// apart. The clock bounds every copy of the database at once: the counter
// starts at 0 and grows far slower than one per microsecond, so no copy reaches
// the time of a later restore, even when the restored backup predates an
// earlier restore and has no record of its jump.
//
// Recording a restore that did not happen, or the same one twice, is harmless:
// every cursor a device holds is then at most the current counter, so the new
// gap holds none of them.
func RecordRestore(ctx context.Context, db *sql.DB, now time.Time) (RestoreGap, error) {
	tx, err := BeginWrite(ctx, db)
	if err != nil {
		return RestoreGap{}, fmt.Errorf("begin restore: %w", err)
	}
	defer tx.Rollback()

	q := New(tx)
	restored, err := q.CurrentVersion(ctx)
	if err != nil {
		return RestoreGap{}, fmt.Errorf("current version: %w", err)
	}
	gap := RestoreGap{
		RestoredVersion: restored,
		JumpedVersion:   max(restored+minRestoreJump, now.UnixMicro()),
		RestoredAt:      now.UTC(),
	}
	if gap.JumpedVersion > maxVersion {
		return RestoreGap{}, fmt.Errorf("a restore at %s would move the sync version to %d, which the PWA cannot read exactly: check the clock",
			now.Format(time.RFC3339), gap.JumpedVersion)
	}
	if err := q.JumpVersion(ctx, gap.JumpedVersion); err != nil {
		return RestoreGap{}, fmt.Errorf("jump version: %w", err)
	}
	if err := q.CreateRestoreGap(ctx, CreateRestoreGapParams(gap)); err != nil {
		return RestoreGap{}, fmt.Errorf("record restore gap: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return RestoreGap{}, fmt.Errorf("commit restore: %w", err)
	}
	return gap, nil
}
