#!/usr/bin/env bash
# Installs the Quitsie binary that arrives on stdin and restarts the service.
#
# provision.sh installs it as /usr/local/sbin/quitsie-deploy. The CI key in
# ~deploy/.ssh/authorized_keys can run nothing else: its forced command is
# `sudo -n /usr/local/sbin/quitsie-deploy`, and sudoers lets the deploy user run
# only this script, without arguments. The binary travels on stdin, so there is
# no path for a caller to point at, and a binary that does not come up healthy
# is rolled back to the previous one.
#
# Migrations run when the new binary starts and only go forward: keep each one
# compatible with the previous release, or the rollback will start an old
# binary against a newer schema.
set -euo pipefail
umask 022

BIN=/usr/local/bin/quitsie
HEALTH_URL=http://127.0.0.1:8080/api/v1/health
MIN_BYTES=$((1024 * 1024))
MAX_BYTES=$((100 * 1024 * 1024))

die() { echo "quitsie-deploy: $*" >&2; exit 1; }

[[ $EUID -eq 0 ]] || die "must run as root (through sudo)"
[[ $# -eq 0 ]] || die "takes no arguments: send the binary on stdin"
[[ ! -t 0 ]] || die "send the binary on stdin"

exec 9>/run/quitsie-deploy.lock
flock -n 9 || die "another deploy is running"

# Same directory as $BIN, so the final mv is an atomic rename.
tmp=$(mktemp "$BIN.new.XXXXXX")
trap 'rm -f "$tmp"' EXIT

head -c $((MAX_BYTES + 1)) > "$tmp"
size=$(stat -c %s "$tmp")
(( size >= MIN_BYTES )) || die "received $size bytes: too small to be the server"
(( size <= MAX_BYTES )) || die "received more than $MAX_BYTES bytes"
[[ $(head -c 4 "$tmp" | od -An -tx1 | tr -d ' \n') == 7f454c46 ]] || die "not an ELF executable"
chmod 0755 "$tmp"
echo "received $size bytes, sha256 $(sha256sum "$tmp" | cut -d' ' -f1)"

# wait_healthy succeeds once the service is up and the health endpoint answers 200.
wait_healthy() {
  local body
  for _ in $(seq 1 30); do
    if systemctl is-active --quiet quitsie && body=$(curl -fsS --max-time 2 "$HEALTH_URL" 2>/dev/null); then
      echo "healthy: $body"
      return 0
    fi
    sleep 1
  done
  return 1
}

previous=""
if [[ -f $BIN ]]; then
  cp -p "$BIN" "$BIN.prev"
  previous=$BIN.prev
fi
mv -f "$tmp" "$BIN"

echo "restarting quitsie"
systemctl restart quitsie
if wait_healthy; then
  echo "deployed"
  exit 0
fi

echo "quitsie-deploy: the new binary did not become healthy" >&2
journalctl -u quitsie -n 40 --no-pager >&2 || true
if [[ -n $previous ]]; then
  echo "rolling back to the previous binary" >&2
  mv -f "$previous" "$BIN"
  systemctl restart quitsie
  if wait_healthy; then
    echo "rolled back: the previous binary is serving again" >&2
  else
    echo "quitsie-deploy: the previous binary is not healthy either; check the VM" >&2
  fi
fi
exit 1
