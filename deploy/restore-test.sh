#!/usr/bin/env bash
# Proves that the latest backup in R2 restores into a healthy database.
#
# provision.sh installs it as /usr/local/sbin/quitsie-restore-test. Run it on
# the VM as root:
#
#   sudo quitsie-restore-test
#
# It restores into a temporary directory and checks the result; the live
# database and the replica are never written. It exits non-zero if anything
# fails, so it can also run from a timer.
set -euo pipefail

CONFIG=/etc/litestream.yml
ENV_FILE=/etc/quitsie/litestream.env
LIVE_DB=/var/lib/quitsie/quitsie.db

die() { echo "restore-test: $*" >&2; exit 1; }

[[ $EUID -eq 0 ]] || die "run it as root (sudo quitsie-restore-test)"
[[ -r $ENV_FILE ]] || die "$ENV_FILE is missing: run provision.sh first"

# The values were validated by provision.sh, so sourcing the file is safe.
set -a
# shellcheck disable=SC1090
. "$ENV_FILE"
set +a

work=$(mktemp -d /var/tmp/quitsie-restore-test.XXXXXX)
trap 'rm -rf "$work"' EXIT
restored=$work/restored.db

echo "restoring the latest backup from R2 into $restored"
litestream restore -config "$CONFIG" -o "$restored" -integrity-check full "$LIVE_DB"

[[ $(sqlite3 "$restored" 'PRAGMA integrity_check;') == ok ]] || die "integrity_check failed on the restored database"

# query DB SQL prints one value, or "n/a" when the table is not there. Read-only,
# so looking at the live database cannot disturb it.
query() { sqlite3 -readonly "$1" "$2" 2>/dev/null || echo "n/a"; }

echo "restored database: $(stat -c %s "$restored") bytes, $(query "$restored" "SELECT count(*) FROM sqlite_master WHERE type = 'table';") tables"
echo "  schema version (goose): $(query "$restored" 'SELECT max(version_id) FROM goose_db_version;')"
echo "  sync version in the backup: $(query "$restored" 'SELECT value FROM version_counter;')"
if [[ -f $LIVE_DB ]]; then
  # A few versions of lag are normal: the backup trails the live database by about a second.
  echo "  sync version in the live database: $(query "$LIVE_DB" 'SELECT value FROM version_counter;')"
fi
echo "OK: the backup restores and passes integrity_check"
