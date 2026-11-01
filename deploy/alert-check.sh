#!/usr/bin/env bash
# Checks that Quitsie is up and emails the owner when something needs their
# attention.
#
# provision.sh installs it as /usr/local/sbin/quitsie-alert-check. Systemd runs
# it as root in two ways:
#   quitsie-alert-check.timer      every 5 minutes, with every check;
#   quitsie-alert@<unit>.service   from OnFailure= of a unit, as soon as
#                                  systemd marks that unit as failed.
# By hand, as root:
#   quitsie-alert-check            runs the checks and says what it found
#   quitsie-alert-check --test     sends a test email
#
# The checks: /api/v1/health answers 200; quitsie, litestream and cloudflared
# are active and did not restart on their own; quitsie-restore and the weekly
# restore test did not fail; the data disk is under 85 %; and no error reached
# the journal of quitsie or litestream since the last run.
#
# A failing check sends one email, then at most one every 4 hours while it
# keeps failing, and a "resolved" email when it passes again. Everything a run
# finds goes in one email. The state lives in /var/lib/quitsie-alert. Without
# QUITSIE_ALERT_EMAIL (in /etc/quitsie/alert.env) it only writes to the journal.
#
# The email goes out through Resend with the app's own key, read from
# /etc/quitsie/quitsie.env. curl gets the key on stdin, never as an argument,
# so it does not show in the process list, and nothing here prints it.
set -euo pipefail
umask 077

# Moves the files this script reads and writes (configuration, state, the app
# binary, the deploy lock) under another directory, for
# deploy/alert-check.test.sh. Nothing sets it on the VM.
ROOT=${QUITSIE_ALERT_TEST_ROOT:-}

APP_ENV=$ROOT/etc/quitsie/quitsie.env
ALERT_ENV=$ROOT/etc/quitsie/alert.env
STATE_DIR=$ROOT/var/lib/quitsie-alert
APP_BIN=$ROOT/usr/local/bin/quitsie
DEPLOY_LOCK=$ROOT/run/quitsie-deploy.lock
DATA_DIR=/var/lib/quitsie
HEALTH_URL=http://127.0.0.1:8080/api/v1/health
RESEND_URL=https://api.resend.com/emails

DISK_MAX_PERCENT=85
REPEAT_SECONDS=$((4 * 3600))
# Alerts share the Resend account with the login codes, and its free plan
# sends 100 emails a day: a storm of alerts must not use them up.
MAX_EMAILS_PER_DAY=10

# Long-running units must be active and must not have restarted on their own;
# oneshot units must not be failed.
LONG_RUNNING=(quitsie.service litestream.service cloudflared.service)
ONESHOT=(quitsie-restore.service quitsie-restore-test.service)
# Units whose journal is searched for errors. Both log with Go's slog text
# handler, so an error line carries level=ERROR. The app logs at ERROR
# whatever needs the owner, 500s included (handlerError in
# api/internal/httpapi/problem.go), and so does net/http for a panic in a
# handler (newHTTPServer in api/cmd/server/main.go). Binaries before that
# logged the panic at INFO, and a crash writes "panic:" or "fatal error:"
# straight to stderr, so those count too.
LOGGED=(quitsie.service litestream.service)
ERROR_RE='level=ERROR|msg="http: panic serving|^panic: |^fatal error: '

log() { printf '%s\n' "$*"; }
die() { printf 'quitsie-alert-check: %s\n' "$*" >&2; exit 1; }

# env_value FILE KEY prints the value that FILE gives KEY, read the way
# systemd's EnvironmentFile= reads the forms these files use: KEY=value,
# KEY="value" and KEY='value'. It prints nothing when either is missing.
env_value() {
  local line value
  [[ -r $1 ]] || return 0
  line=$(grep -E "^[[:space:]]*$2=" "$1" | tail -n 1) || return 0
  value=${line#*=}
  value=${value#"${value%%[![:space:]]*}"}
  value=${value%"${value##*[![:space:]]}"}
  if [[ $value =~ ^\"(.*)\"$ || $value =~ ^\'(.*)\'$ ]]; then
    value=${BASH_REMATCH[1]}
  fi
  printf '%s' "$value"
}

# read_number FILE prints the number stored in FILE, or 0.
read_number() {
  local n=""
  if [[ -r $1 ]]; then n=$(<"$1"); fi
  if [[ $n =~ ^[0-9]+$ ]]; then printf '%s' "$n"; else printf '0'; fi
}

declare -A FAILED=() # check -> what is wrong, in Spanish, for the email
PASSED=()
NEW_ERRORS=""

fail() { FAILED[$1]=$2; }
pass() { PASSED+=("$1"); }

# deploy_running succeeds while quitsie-deploy holds its lock: the app is
# restarting on purpose, and the deploy rolls back by itself if it fails.
deploy_running() {
  [[ -e $DEPLOY_LOCK ]] || return 1
  ! (exec 8<"$DEPLOY_LOCK" && flock -n 8)
}

check_health() {
  local code="" try
  for try in 1 2 3; do
    code=$(curl -q -sS -o "$WORK/health.json" -w '%{http_code}' --max-time 5 "$HEALTH_URL" 2>/dev/null) || true
    if [[ $code == 200 && $(jq -r '.status // empty' "$WORK/health.json" 2>/dev/null) == ok ]]; then
      pass health
      return
    fi
    if (( try < 3 )); then sleep 5; fi
  done
  fail health "La aplicación no responde: GET /api/v1/health devolvió ${code:-000} tres veces seguidas (000 es que no contestó). Mira: journalctl -u quitsie -n 100"
}

# unit_state UNIT prints its active state. A unit in transition (a restart,
# a deploy) gets about 10 seconds to settle.
unit_state() {
  local state="" try
  for try in 1 2 3; do
    state=$(systemctl is-active "$1" 2>/dev/null) || true
    case $state in
      activating|deactivating|reloading|refreshing) if (( try < 3 )); then sleep 5; fi ;;
      *) break ;;
    esac
  done
  printf '%s' "${state:-unknown}"
}

check_units() {
  local unit state restarts last file f
  for unit in "${LONG_RUNNING[@]}"; do
    if [[ $unit == quitsie.service ]] && { [[ ! -x $APP_BIN ]] || deploy_running; }; then
      continue
    fi
    state=$(unit_state "$unit")
    # NRestarts counts the restarts systemd made on its own after a crash;
    # a manual restart sets it back to zero.
    restarts=$(systemctl show -p NRestarts --value "$unit" 2>/dev/null) || restarts=""
    file=$STATE_DIR/$unit.restarts
    last=-1
    if [[ -f $file ]]; then last=$(read_number "$file"); fi
    if [[ $restarts =~ ^[0-9]+$ ]]; then printf '%s\n' "$restarts" > "$file"; fi

    if [[ $state != active ]]; then
      fail "unit-$unit" "$unit está $state. Mira: journalctl -u $unit -n 100"
    elif [[ $restarts =~ ^[0-9]+$ ]] && (( last >= 0 && restarts > last )); then
      fail "unit-$unit" "$unit se cayó y systemd lo volvió a arrancar $((restarts - last)) vez/veces desde la comprobación anterior. Mira: journalctl -u $unit -n 100"
    else
      pass "unit-$unit"
    fi
  done

  # Besides the oneshot units, any unit an OnFailure= reported is followed
  # until it is no longer failed.
  local units=("${ONESHOT[@]}")
  for f in "$STATE_DIR"/unit-*.failing; do
    [[ -e $f ]] || continue
    unit=${f##*/unit-}
    unit=${unit%.failing}
    case " ${LONG_RUNNING[*]} ${units[*]} " in
      *" $unit "*) ;;
      *) units+=("$unit") ;;
    esac
  done
  for unit in "${units[@]}"; do
    if systemctl is-failed --quiet "$unit" 2>/dev/null; then
      fail "unit-$unit" "$(failed_message "$unit")"
    else
      pass "unit-$unit"
    fi
  done
}

failed_message() {
  case $1 in
    quitsie-restore-test.service)
      echo "La prueba de restauración del respaldo falló: puede que el respaldo de R2 no sirva. Mira: journalctl -u quitsie-restore-test -n 100. Cuando lo arregles, repítela con: sudo systemctl start quitsie-restore-test" ;;
    quitsie-restore.service)
      echo "quitsie-restore falló al arrancar, así que ni la aplicación ni Litestream arrancan. Mira: journalctl -u quitsie-restore -n 100" ;;
    *)
      echo "systemd marcó $1 como fallida. Mira: journalctl -u $1 -n 100" ;;
  esac
}

check_disk() {
  local dir=$DATA_DIR pct
  [[ -d $dir ]] || dir=/
  pct=$(df -P "$dir" 2>/dev/null | awk 'NR == 2 { sub(/%$/, "", $5); print $5 }') || pct=""
  if [[ ! $pct =~ ^[0-9]+$ ]]; then
    fail disk "No pude leer con df cuánto disco queda en $dir."
  elif (( pct >= DISK_MAX_PERCENT )); then
    fail disk "El disco de $dir está al $pct % (el aviso salta al $DISK_MAX_PERCENT %). Mira qué ocupa: sudo du -xh -d1 /var | sort -h"
  else
    pass disk
  fi
}

# summarize reads log lines and prints their messages, most frequent first.
# Only the msg of a slog line is kept: its other fields can hold paths,
# addresses or error texts that have no place in an email. The client
# address of a panic goes too, so that the same panic counts as one.
# awk, unlike head, reads all its input: an early exit would kill sort with
# SIGPIPE, and pipefail would end the whole run when errors pour in.
summarize() {
  sed -E -e 's/^.*[[:space:]]msg=("([^"\\]|\\.)*"|[^[:space:]]*).*$/msg=\1/' \
    -e 's/(http: panic serving )[^ ]+: /\1/' \
    | cut -c 1-160 | sort | uniq -c | sort -rn \
    | awk 'NR <= 5 { sub(/^ *[0-9]+ /, "  " $1 " × "); print }'
}

check_errors() {
  local unit cursor out lines count
  local -a args
  for unit in "${LOGGED[@]}"; do
    cursor=$STATE_DIR/$unit.cursor
    # journalctl starts after the cursor in that file and leaves there the
    # cursor of the last line it printed. The first run looks back 10 minutes.
    args=(--unit="$unit" --output=cat --no-pager --quiet --cursor-file="$cursor")
    if [[ ! -s $cursor ]]; then args+=(--since=-10min); fi
    if ! out=$(journalctl "${args[@]}" 2>/dev/null); then
      rm -f "$cursor"
      log "errors: could not read the journal of $unit; the next run starts over"
      continue
    fi
    lines=$(grep -E "$ERROR_RE" <<<"$out") || continue
    count=$(grep -c '' <<<"$lines")
    NEW_ERRORS+="$unit: $count línea(s) de error. Las más repetidas:"$'\n'
    NEW_ERRORS+=$(summarize <<<"$lines")$'\n'
  done
}

# resolved_message CHECK says, in the email, that CHECK passes again.
resolved_message() {
  case $1 in
    health) echo "La aplicación responde otra vez." ;;
    disk) echo "El disco bajó del $DISK_MAX_PERCENT %." ;;
    unit-*) echo "${1#unit-} funciona otra vez." ;;
    *) echo "$1 funciona otra vez." ;;
  esac
}

send_email() {
  local subject=$1 text=$2 key from code day sent
  key=$(env_value "$APP_ENV" QUITSIE_RESEND_API_KEY)
  from=$(env_value "$APP_ENV" QUITSIE_MAIL_FROM)
  if [[ -z $key || -z $from ]]; then
    log "cannot email: QUITSIE_RESEND_API_KEY or QUITSIE_MAIL_FROM has no value in $APP_ENV"
    return 1
  fi
  day=$((NOW / 86400))
  sent=$(read_number "$STATE_DIR/emails.$day")
  if (( sent >= MAX_EMAILS_PER_DAY )); then
    log "cannot email: $sent alert emails went out today, the most a day allows; it tries again tomorrow"
    return 1
  fi
  jq -n --arg from "$from" --arg to "$ALERT_EMAIL" --arg subject "$subject" --arg text "$text" \
    '{from: $from, to: [$to], subject: $subject, text: $text}' > "$WORK/email.json"
  code=$(printf 'Authorization: Bearer %s\n' "$key" \
    | curl -q -sS --max-time 20 -o "$WORK/resend.json" -w '%{http_code}' -H @- \
        -H 'Content-Type: application/json' --data-binary @"$WORK/email.json" "$RESEND_URL" 2>"$WORK/curl.err") || true
  if [[ $code != 2[0-9][0-9] ]]; then
    log "cannot email: Resend answered ${code:-nothing}: $(jq -r '.message // empty' "$WORK/resend.json" 2>/dev/null | head -c 200)$(head -c 200 "$WORK/curl.err")"
    return 1
  fi
  find "$STATE_DIR" -maxdepth 1 -name 'emails.*' ! -name "emails.$day" -delete
  printf '%s\n' "$((sent + 1))" > "$STATE_DIR/emails.$day"
  log "emailed $ALERT_EMAIL: $subject"
}

# notify decides what deserves an email, sends one with all of it, and
# records what was said so the next runs neither repeat nor forget it.
notify() {
  local check marker emailed due=0 errors_due=0 subject body
  local -a failing=() resolved=()

  if (( ${#FAILED[@]} )); then
    mapfile -t failing < <(printf '%s\n' "${!FAILED[@]}" | sort)
  fi
  for check in "${failing[@]}"; do
    log "FAIL $check: ${FAILED[$check]}"
    marker=$STATE_DIR/$check.failing
    emailed=$(read_number "$STATE_DIR/$check.emailed")
    # The marker exists while the check fails, and says "notified" once an
    # email told the owner. A check that fails again soon after an email
    # waits for the end of REPEAT_SECONDS, like a reminder.
    if [[ ! -f $marker ]]; then : > "$marker"; fi
    if (( NOW - emailed >= REPEAT_SECONDS )); then
      due=1
    else
      log "$check: the last email about it went out $(( (NOW - emailed) / 60 )) minutes ago; not repeated yet"
    fi
  done
  for check in "${PASSED[@]}"; do
    marker=$STATE_DIR/$check.failing
    [[ -f $marker ]] || continue
    if [[ $(<"$marker") == notified ]]; then
      resolved+=("$check")
      due=1
    else
      rm -f "$marker"
    fi
    log "OK again $check"
  done

  # Errors are events, not states: they pile up and go out together, at
  # most once every REPEAT_SECONDS, and there is no "resolved" for them.
  local errors_file=$STATE_DIR/errors.pending
  if [[ -n $NEW_ERRORS ]]; then
    printf '%s' "$NEW_ERRORS"
    printf '%s' "$NEW_ERRORS" >> "$errors_file"
    tail -n 60 "$errors_file" > "$errors_file.tmp" && mv -f "$errors_file.tmp" "$errors_file"
  fi
  if [[ -s $errors_file ]]; then
    emailed=$(read_number "$STATE_DIR/errors.emailed")
    if (( NOW - emailed >= REPEAT_SECONDS )); then
      errors_due=1
      due=1
    else
      log "errors: held back until $(( (REPEAT_SECONDS - NOW + emailed) / 60 )) minutes from now, so they do not flood the inbox"
    fi
  fi

  (( due )) || return 0

  if (( ${#failing[@]} || errors_due )); then
    subject="Quitsie: algo necesita tu atención"
  else
    subject="Quitsie: todo vuelve a funcionar"
  fi
  body="Aviso de quitsie-alert-check en la VM $(uname -n), $(date -u -d "@$NOW" '+%Y-%m-%d %H:%M UTC')."$'\n'
  if (( ${#failing[@]} )); then
    body+=$'\n'"Falla:"$'\n'
    for check in "${failing[@]}"; do
      body+="- ${FAILED[$check]}"
      if [[ $(<"$STATE_DIR/$check.failing") == notified ]]; then body+=" (sigue desde el aviso anterior)"; fi
      body+=$'\n'
    done
  fi
  if [[ -s $errors_file ]]; then
    body+=$'\n'"Errores en el registro desde el último aviso:"$'\n'"$(<"$errors_file")"$'\n'
    body+="Míralos con: journalctl -u quitsie -u litestream --since -4h | grep -E 'level=ERROR|panic'"$'\n'
  fi
  if (( ${#resolved[@]} )); then
    body+=$'\n'"Resuelto:"$'\n'
    for check in "${resolved[@]}"; do body+="- $(resolved_message "$check")"$'\n'; done
  fi
  body+=$'\n'
  if (( ${#failing[@]} )); then
    body+="Mientras algo siga fallando, este aviso se repite como mucho cada $((REPEAT_SECONDS / 3600)) horas."$'\n'
  fi
  body+="Cómo entrar a la VM y qué mirar: docs/despliegue.md, sección «Alertas»."$'\n'

  send_email "$subject" "$body" || return 1

  for check in "${failing[@]}"; do
    echo notified > "$STATE_DIR/$check.failing"
    printf '%s\n' "$NOW" > "$STATE_DIR/$check.emailed"
  done
  for check in "${resolved[@]}"; do rm -f "$STATE_DIR/$check.failing"; done
  if [[ -s $errors_file ]]; then
    rm -f "$errors_file"
    printf '%s\n' "$NOW" > "$STATE_DIR/errors.emailed"
  fi
}

mode=check
failed_unit=""
case ${1:-} in
  "") ;;
  --test) mode=send-test ;;
  --failed)
    mode=failed
    failed_unit=${2:-}
    # From OnFailure=quitsie-alert@%n.service, through the instance name.
    [[ $failed_unit =~ ^[A-Za-z0-9@._:-]+\.(service|timer|socket|mount)$ ]] || die "--failed takes a unit name, not '$failed_unit'"
    ;;
  *) die "usage: quitsie-alert-check [--test | --failed UNIT]" ;;
esac

[[ $EUID -eq 0 || -n $ROOT ]] || die "run it as root (sudo quitsie-alert-check)"
for tool in curl jq flock journalctl systemctl; do
  command -v "$tool" >/dev/null || die "$tool is missing: run provision.sh again"
done

mkdir -p "$STATE_DIR" # 0700, by the umask above
exec 9>"$STATE_DIR/lock"
flock -w 120 9 || die "another check has been running for two minutes"
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT
NOW=$(date +%s)

ALERT_EMAIL=$(env_value "$ALERT_ENV" QUITSIE_ALERT_EMAIL)
if [[ -n $ALERT_EMAIL && ! $ALERT_EMAIL =~ ^[^@[:space:]]+@[^@[:space:]]+\.[^@[:space:]]+$ ]]; then
  log "QUITSIE_ALERT_EMAIL in $ALERT_ENV is not an email address; nothing is emailed"
  ALERT_EMAIL=""
fi

if [[ $mode == send-test ]]; then
  [[ -n $ALERT_EMAIL ]] || die "QUITSIE_ALERT_EMAIL has no value: sudoedit $ALERT_ENV"
  send_email "Quitsie: correo de prueba de las alertas" \
    "Si lees esto, la VM $(uname -n) sabe avisarte. No hay que hacer nada."$'\n' \
    || die "the test email was not sent"
  exit 0
fi

if [[ $mode == failed ]]; then
  fail "unit-$failed_unit" "$(failed_message "$failed_unit")"
else
  if [[ ! -x $APP_BIN ]]; then
    log "health: skipped until the first deploy installs $APP_BIN"
  elif deploy_running; then
    log "health: skipped while a deploy is running"
  else
    check_health
  fi
  check_units
  check_disk
  check_errors
fi

if [[ -z $ALERT_EMAIL ]]; then
  for check in "${!FAILED[@]}"; do log "FAIL $check: ${FAILED[$check]}"; done
  if [[ -n $NEW_ERRORS ]]; then printf '%s' "$NEW_ERRORS"; fi
  log "QUITSIE_ALERT_EMAIL has no value in $ALERT_ENV: this goes only to the journal"
  exit 0
fi

notify || exit 1
log "checked: ${#FAILED[@]} failing, ${#PASSED[@]} passing"
