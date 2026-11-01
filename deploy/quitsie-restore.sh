#!/usr/bin/env bash
# Brings the Quitsie database back from R2 when the VM has none, and tells the
# app that it did.
#
# provision.sh installs it as /usr/local/sbin/quitsie-restore, and
# quitsie-restore.service runs it once per boot as the quitsie user, before the
# app and Litestream:
#
#   quitsie-restore CONFIG DB
#
# It does nothing when DB exists, or when R2 has no backup yet (the very first
# deploy). If R2 cannot be reached it fails.
#
# After a restore it leaves the marker DB.restored. On its next start the app
# makes the sync versions jump past those the lost database handed out, so
# devices that synced after the backup rebuild their replicas, and removes the
# marker (docs/arquitectura.md, "Restaurar un respaldo"). The marker goes down
# before the restore and is removed only if no database came back: a failure
# at any point can leave a marker without a restore, which costs nothing, but
# never a restore without its marker.
set -euo pipefail

config=${1:?usage: quitsie-restore CONFIG DB}
db=${2:?usage: quitsie-restore CONFIG DB}
marker=$db.restored

if [[ -e $db ]]; then
  echo "quitsie-restore: $db exists, nothing to restore"
  exit 0
fi

: > "$marker"
litestream restore -config "$config" -if-db-not-exists -if-replica-exists "$db"

if [[ -e $db ]]; then
  echo "quitsie-restore: restored $db from the backup; the app records the restore when it starts"
else
  rm -f "$marker"
  echo "quitsie-restore: no backup yet, so the app starts with a new database"
fi
