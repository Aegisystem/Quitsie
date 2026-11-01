#!/usr/bin/env bash
# Tests deploy/alert-check.sh against stand-ins for curl, systemctl,
# journalctl, df, date and sleep: no root, no VM and no email leave this
# machine. CI runs it.
#
#   bash deploy/alert-check.test.sh
set -euo pipefail

SCRIPT=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/alert-check.sh
T=$(mktemp -d)
trap 'rm -rf "$T"' EXIT
STUB=$T/stub # the stand-ins
FAKE=$T/fake # what they answer, and what they saw
ROOT=$T/root # QUITSIE_ALERT_TEST_ROOT
KEY=re_test_SECRET_123456

mkdir -p "$STUB"
cat > "$STUB/curl" <<'EOF'
#!/usr/bin/env bash
# Answers the health URL and the Resend API from $FAKE, and keeps every email.
set -euo pipefail
printf '%s\n' "$*" >> "$FAKE/curl.argv"
out="" data="" url="" stdin_headers=0
while (( $# )); do
  case $1 in
    -o) out=$2; shift ;;
    -w|--max-time) shift ;;
    --data-binary) data=${2#@}; shift ;;
    -H) if [[ $2 == @- ]]; then stdin_headers=1; fi; shift ;;
    -*) ;;
    *) url=$1 ;;
  esac
  shift
done
case $url in
  http://127.0.0.1:8080/api/v1/health)
    code=$(<"$FAKE/health")
    if [[ $code == 000 ]]; then printf '000'; exit 7; fi
    if [[ $code == 200 ]]; then echo '{"status":"ok","database":"ok"}' > "$out"; else echo '{"status":"error"}' > "$out"; fi
    printf '%s' "$code"
    ;;
  https://api.resend.com/emails)
    code=$(<"$FAKE/resend")
    if [[ $code == 200 ]]; then
      n=$(( $(cat "$FAKE/sent/count" 2>/dev/null || echo 0) + 1 ))
      echo "$n" > "$FAKE/sent/count"
      cp "$data" "$FAKE/sent/$n.json"
      if (( stdin_headers )); then cat > "$FAKE/sent/$n.headers"; fi
      echo '{"id":"x"}' > "$out"
    else
      echo '{"message":"Resend is down"}' > "$out"
    fi
    printf '%s' "$code"
    ;;
  *) echo "unexpected URL $url" >&2; exit 2 ;;
esac
EOF
cat > "$STUB/systemctl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
case $1 in
  is-active)
    state=$(cat "$FAKE/units/$2.state" 2>/dev/null || echo active)
    echo "$state"
    [[ $state == active ]] ;;
  is-failed)
    [[ -e $FAKE/units/${!#}.failed ]] ;;
  show)
    cat "$FAKE/units/${!#}.restarts" 2>/dev/null || echo 0 ;;
  *) exit 2 ;;
esac
EOF
cat > "$STUB/journalctl" <<'EOF'
#!/usr/bin/env bash
# Prints the lines of $FAKE/journal/<unit>.log after the cursor, which is a
# line number, and moves the cursor like --cursor-file does.
set -euo pipefail
unit="" cursor=""
for a in "$@"; do
  case $a in
    --unit=*) unit=${a#--unit=} ;;
    --cursor-file=*) cursor=${a#--cursor-file=} ;;
  esac
done
log=$FAKE/journal/$unit.log
[[ -f $log ]] || exit 0
start=0
if [[ -s $cursor ]]; then start=$(<"$cursor"); fi
total=$(wc -l < "$log")
if (( total > start )); then
  tail -n "+$((start + 1))" "$log"
  echo "$total" > "$cursor"
fi
EOF
cat > "$STUB/df" <<'EOF'
#!/usr/bin/env bash
echo "Filesystem 1024-blocks Used Available Capacity Mounted on"
echo "/dev/sda1 30000000 1 1 $(<"$FAKE/disk")% /"
EOF
cat > "$STUB/date" <<'EOF'
#!/usr/bin/env bash
if [[ $* == +%s ]]; then cat "$FAKE/now"; else echo "2026-11-01 12:00 UTC"; fi
EOF
printf '#!/bin/sh\nexit 0\n' > "$STUB/sleep"
chmod +x "$STUB"/*

failures=0
current=""

reset() {
  rm -rf "$FAKE" "$ROOT"
  mkdir -p "$FAKE/units" "$FAKE/journal" "$FAKE/sent" "$ROOT/etc/quitsie" "$ROOT/usr/local/bin" "$ROOT/run"
  echo 1800000000 > "$FAKE/now"
  echo 200 > "$FAKE/health"
  echo 200 > "$FAKE/resend"
  echo 40 > "$FAKE/disk"
  : > "$FAKE/curl.argv"
  printf 'QUITSIE_RESEND_API_KEY=%s\nQUITSIE_MAIL_FROM="Quitsie <codigos@example.com>"\n' "$KEY" > "$ROOT/etc/quitsie/quitsie.env"
  echo 'QUITSIE_ALERT_EMAIL=owner@example.com' > "$ROOT/etc/quitsie/alert.env"
  printf '#!/bin/sh\n' > "$ROOT/usr/local/bin/quitsie"
  chmod +x "$ROOT/usr/local/bin/quitsie"
}

test_case() {
  current=$1
  reset
}

# check [ARGS...] runs the script once; its output is in $T/out, its exit status in $status.
check() {
  status=0
  QUITSIE_ALERT_TEST_ROOT=$ROOT FAKE=$FAKE PATH="$STUB:$PATH" bash "$SCRIPT" "$@" > "$T/out" 2>&1 || status=$?
  cat "$T/out" >> "$T/all-output"
}

advance() { echo $(( $(<"$FAKE/now") + $1 * 60 )) > "$FAKE/now"; }
emails() { cat "$FAKE/sent/count" 2>/dev/null || echo 0; }
last_email() { jq -r ".$1" "$FAKE/sent/$(emails).json"; }

bad() {
  echo "FAIL [$current] $*"
  sed 's/^/    | /' "$T/out"
  failures=$((failures + 1))
}
expect_emails() { [[ $(emails) == "$1" ]] || bad "expected $1 email(s), got $(emails)"; }
expect_status() { [[ $status == "$1" ]] || bad "expected exit status $1, got $status"; }
expect_out() { grep -qF -- "$1" "$T/out" || bad "output lacks: $1"; }
expect_subject() { [[ $(last_email subject) == *"$1"* ]] || bad "subject '$(last_email subject)' lacks: $1"; }
expect_text() { [[ $(last_email text) == *"$1"* ]] || bad "email lacks: $1"; }
expect_no_text() { [[ $(last_email text) != *"$1"* ]] || bad "email should not have: $1"; }

test_case "all fine: no email"
check
expect_status 0
expect_emails 0
expect_out "checked: 0 failing"

test_case "the app is down: one email, silence, a reminder, then resolved"
echo 000 > "$FAKE/health"
check
expect_status 0
expect_emails 1
expect_subject "algo necesita tu atención"
expect_text "La aplicación no responde"
[[ $(last_email from) == "Quitsie <codigos@example.com>" ]] || bad "from is $(last_email from)"
[[ $(jq -r '.to[0]' "$FAKE/sent/1.json") == owner@example.com ]] || bad "wrong recipient"
advance 5; check
expect_emails 1
expect_out "not repeated yet"
advance 240; check
expect_emails 2
expect_text "(sigue desde el aviso anterior)"
echo 200 > "$FAKE/health"
advance 5; check
expect_emails 3
expect_subject "todo vuelve a funcionar"
expect_text "La aplicación responde otra vez."
advance 5; check
expect_emails 3

test_case "a health answer other than 200 is a failure"
echo 503 > "$FAKE/health"
check
expect_emails 1
expect_text "devolvió 503"

test_case "litestream stopped"
echo inactive > "$FAKE/units/litestream.service.state"
check
expect_emails 1
expect_text "litestream.service está inactive"

test_case "cloudflared failed"
echo failed > "$FAKE/units/cloudflared.service.state"
check
expect_emails 1
expect_text "cloudflared.service está failed"

test_case "a unit that restarted on its own, then stays up"
echo 5 > "$FAKE/units/quitsie.service.restarts"
check
expect_emails 0 # the first run only takes note of the count
echo 7 > "$FAKE/units/quitsie.service.restarts"
advance 5; check
expect_emails 1
expect_text "quitsie.service se cayó y systemd lo volvió a arrancar 2 vez/veces"
advance 5; check
expect_emails 2
expect_subject "todo vuelve a funcionar"
echo 0 > "$FAKE/units/quitsie.service.restarts" # a manual restart resets it
advance 5; check
expect_emails 2

test_case "disk at the threshold, then below"
echo 85 > "$FAKE/disk"
check
expect_emails 1
expect_text "está al 85 %"
echo 84 > "$FAKE/disk"
advance 5; check
expect_emails 2
expect_text "El disco bajó del 85 %."

test_case "error lines: one email, held back, then sent; never their details"
{
  echo 'time=2026-11-01T12:00:00Z level=INFO msg=listening url=http://127.0.0.1:8080'
  echo 'time=2026-11-01T12:00:01Z level=ERROR msg="handler failed" method=GET path=/api/v1/sync err="detail of someone@example.org"'
  echo 'time=2026-11-01T12:00:02Z level=ERROR msg="handler failed" method=POST path=/api/v1/records err="other"'
  echo 'time=2026-11-01T12:00:03Z level=INFO msg="http: panic serving 127.0.0.1:5555: boom"'
  echo 'time=2026-11-01T12:00:04Z level=WARN msg="Google sign-in failed" err=x'
} > "$FAKE/journal/quitsie.service.log"
echo 'time=2026-11-01T12:00:05Z level=ERROR msg="sync error" db=/var/lib/quitsie/quitsie.db' > "$FAKE/journal/litestream.service.log"
check
expect_emails 1
expect_text "quitsie.service: 3 línea(s) de error"
expect_text '2 × msg="handler failed"'
expect_text 'msg="http: panic serving 127.0.0.1:5555: boom"'
expect_text "litestream.service: 1 línea(s) de error"
expect_no_text "someone@example.org"
expect_no_text "Google sign-in failed"
advance 5; check
expect_emails 1 # no new lines, and errors have no "resolved"
echo 'time=2026-11-01T12:10:00Z level=ERROR msg="server stopped" err=x' >> "$FAKE/journal/quitsie.service.log"
advance 5; check
expect_emails 1
expect_out "held back"
advance 240; check
expect_emails 2
expect_text 'msg="server stopped"'
advance 5; check
expect_emails 2

test_case "the weekly restore test failed, then passed"
touch "$FAKE/units/quitsie-restore-test.service.failed"
check
expect_emails 1
expect_text "La prueba de restauración del respaldo falló"
rm "$FAKE/units/quitsie-restore-test.service.failed"
advance 5; check
expect_emails 2
expect_text "quitsie-restore-test.service funciona otra vez."

test_case "quitsie-restore failed at boot"
touch "$FAKE/units/quitsie-restore.service.failed"
check
expect_emails 1
expect_text "quitsie-restore falló al arrancar"

test_case "OnFailure: an email at once, resolved by the timer"
check --failed quitsie.service
expect_status 0
expect_emails 1
expect_text "systemd marcó quitsie.service como fallida"
advance 5; check
expect_emails 2
expect_text "quitsie.service funciona otra vez."

test_case "OnFailure of another unit is followed until it is no longer failed"
touch "$FAKE/units/foo.service.failed"
check --failed foo.service
expect_emails 1
advance 5; check
expect_emails 1
rm "$FAKE/units/foo.service.failed"
advance 5; check
expect_emails 2
expect_text "foo.service funciona otra vez."

test_case "--failed refuses what is not a unit name"
check --failed '../../etc/passwd'
expect_status 1
expect_emails 0

test_case "several problems go in one email"
echo 000 > "$FAKE/health"
echo 95 > "$FAKE/disk"
check
expect_emails 1
expect_text "La aplicación no responde"
expect_text "está al 95 %"

test_case "no QUITSIE_ALERT_EMAIL: journal only"
echo 'QUITSIE_ALERT_EMAIL=' > "$ROOT/etc/quitsie/alert.env"
echo 000 > "$FAKE/health"
check
expect_status 0
expect_emails 0
expect_out "FAIL health"
expect_out "this goes only to the journal"
grep -q api.resend.com "$FAKE/curl.argv" && bad "it called Resend"
compgen -G "$ROOT/var/lib/quitsie-alert/*.failing" >/dev/null && bad "it kept alert state"

test_case "Resend fails: exit 1, and the next run tries again"
echo 000 > "$FAKE/health"
echo 500 > "$FAKE/resend"
check
expect_status 1
expect_emails 0
expect_out "Resend answered 500: Resend is down"
echo 200 > "$FAKE/resend"
advance 5; check
expect_status 0
expect_emails 1

test_case "no Resend key yet: exit 1 and say so"
echo 'QUITSIE_MAIL_FROM="Quitsie <codigos@example.com>"' > "$ROOT/etc/quitsie/quitsie.env"
echo 000 > "$FAKE/health"
check
expect_status 1
expect_out "QUITSIE_RESEND_API_KEY or QUITSIE_MAIL_FROM has no value"

test_case "the daily cap holds emails back"
echo 000 > "$FAKE/health"
mkdir -p "$ROOT/var/lib/quitsie-alert"
echo 10 > "$ROOT/var/lib/quitsie-alert/emails.$(( $(<"$FAKE/now") / 86400 ))"
check
expect_emails 0
expect_out "the most a day allows"

test_case "a deploy in progress skips the app"
echo 2 > "$FAKE/units/quitsie.service.restarts"
check
expect_emails 0
echo 000 > "$FAKE/health"
echo activating > "$FAKE/units/quitsie.service.state"
echo 9 > "$FAKE/units/quitsie.service.restarts"
# Holds the lock the way quitsie-deploy does, for the length of one run.
exec 7>"$ROOT/run/quitsie-deploy.lock"
flock 7
advance 5; check
exec 7>&-
expect_emails 0
expect_out "health: skipped while a deploy is running"

test_case "before the first deploy the app is not checked"
rm "$ROOT/usr/local/bin/quitsie"
echo 000 > "$FAKE/health"
echo inactive > "$FAKE/units/quitsie.service.state"
check
expect_emails 0
expect_out "health: skipped until the first deploy"

test_case "--test sends one email"
check --test
expect_status 0
expect_emails 1
expect_subject "correo de prueba"

test_case "the Resend key never shows in arguments or output"
echo 000 > "$FAKE/health"
check
expect_emails 1
grep -qF "$KEY" "$FAKE/curl.argv" && bad "the key was in curl's arguments"
grep -qF "Authorization: Bearer $KEY" "$FAKE/sent/1.headers" || bad "the key did not reach curl on stdin"
grep -qF "$KEY" "$T/all-output" && bad "the key was printed"

if (( failures )); then
  echo "alert-check.test.sh: $failures failure(s)"
  exit 1
fi
echo "alert-check.test.sh: all passed"
