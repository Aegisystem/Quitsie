#!/usr/bin/env bash
# Provisions the Quitsie VM: Debian 12/13 or Ubuntu LTS on amd64. Run it as root
# from a checkout of this repository, on the VM:
#
#   sudo QUITSIE_DEPLOY_PUBKEY='ssh-ed25519 AAAA... quitsie-deploy' \
#     QUITSIE_ALERT_EMAIL=you@example.com bash deploy/provision.sh
#
# Idempotent: run it again to repair the VM or to pick up a newer deploy/. It
# installs cloudflared and Litestream, creates the users and directories,
# installs the systemd units, the deploy script and the alert check, and
# enables everything.
#
# The secrets that must live only on this VM are typed in when it asks (hidden
# input) and go to /etc/quitsie/*.env with mode 600. They are never passed as
# arguments, where they would end up in the shell history, and never printed.
# A file that already has its values is left alone: edit it with sudoedit.
#
# Run it through `gcloud compute ssh --tunnel-through-iap`, not through the
# Cloudflare tunnel: it restarts cloudflared when the tunnel token changes.
#
# QUITSIE_DEPLOY_PUBKEY is the public half of the key GitHub Actions deploys
# with, and QUITSIE_ALERT_EMAIL the owner's address for the alerts. Neither is
# a secret, so they are fine on the command line. Without them, a run keeps
# the values a previous run installed.
set -euo pipefail

# Pinned: bump both together. The digest comes from the release's asset
# (`gh api repos/benbjohnson/litestream/releases/tags/v$LITESTREAM_VERSION`).
LITESTREAM_VERSION=0.5.17
LITESTREAM_SHA256=a191a0928884d1820fab1f866ede1d0d5811c323d0587bf863a43481a82a7668
LITESTREAM_URL=https://github.com/benbjohnson/litestream/releases/download/v${LITESTREAM_VERSION}/litestream-${LITESTREAM_VERSION}-linux-x86_64.deb

DEPLOY_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
ETC_DIR=/etc/quitsie
UNITS=(quitsie.service quitsie-restore.service litestream.service cloudflared.service)
# Units that only watch the others: changing them restarts nothing else.
WATCH_UNITS=(quitsie-alert-check.service quitsie-alert-check.timer quitsie-alert@.service)
TIMERS=(quitsie-alert-check.timer)
# Plain addresses only: the wizard passes QUITSIE_ALERT_EMAIL inside a remote
# command, so no quote or space may get through.
EMAIL_RE='^[A-Za-z0-9._%+-]+@([A-Za-z0-9-]+\.)+[A-Za-z]{2,}$'

CHANGED_UNITS=0      # a unit or litestream.yml differs from what was installed
CHANGED_TIMERS=0     # one of WATCH_UNITS differs from what was installed
NEW_TUNNEL_TOKEN=0   # cloudflared.env was written in this run
NEW_R2_KEYS=0        # litestream.env was written in this run

log()  { printf '\n==> %s\n' "$*"; }
note() { printf '    %s\n' "$*"; }
warn() { printf '    AVISO: %s\n' "$*" >&2; }
die()  { printf 'provision.sh: %s\n' "$*" >&2; exit 1; }

check_platform() {
  [[ $EUID -eq 0 ]] || die "ejecútalo como root: sudo bash deploy/provision.sh"
  [[ $(uname -m) == x86_64 ]] || die "esta guía es para amd64; la arquitectura es $(uname -m)"
  # shellcheck disable=SC1091
  . /etc/os-release
  case ${ID:-} in
    debian|ubuntu) ;;
    *) die "solo se probó en Debian y Ubuntu; esta VM es ${ID:-desconocida}" ;;
  esac
  for f in "${UNITS[@]}" "${WATCH_UNITS[@]}" litestream.yml quitsie-deploy.sh restore-test.sh alert-check.sh \
    quitsie.env.example alert.env.example; do
    [[ -f $DEPLOY_DIR/$f ]] || die "falta $DEPLOY_DIR/$f: ejecútalo desde un checkout completo de deploy/"
  done
}

install_packages() {
  log "Paquetes base"
  export DEBIAN_FRONTEND=noninteractive
  apt-get update -qq
  # sqlite3 is for the restore test, jq for the alert check; unattended-upgrades
  # applies security fixes.
  apt-get install -y -qq --no-install-recommends ca-certificates curl jq sqlite3 sudo unattended-upgrades >/dev/null
  printf 'APT::Periodic::Update-Package-Lists "1";\nAPT::Periodic::Unattended-Upgrade "1";\n' \
    > /etc/apt/apt.conf.d/20auto-upgrades
}

install_cloudflared() {
  log "cloudflared (repositorio apt de Cloudflare)"
  install -d -m 0755 /usr/share/keyrings
  if [[ ! -s /usr/share/keyrings/cloudflare-main.gpg ]]; then
    curl -fsSL https://pkg.cloudflare.com/cloudflare-main.gpg -o /usr/share/keyrings/cloudflare-main.gpg
  fi
  echo 'deb [signed-by=/usr/share/keyrings/cloudflare-main.gpg] https://pkg.cloudflare.com/cloudflared any main' \
    > /etc/apt/sources.list.d/cloudflared.list
  apt-get update -qq
  apt-get install -y -qq cloudflared >/dev/null
  note "$(cloudflared --version | head -n1)"
}

install_litestream() {
  log "Litestream $LITESTREAM_VERSION"
  if command -v litestream >/dev/null && litestream version 2>/dev/null | grep -q "$LITESTREAM_VERSION"; then
    note "ya instalado"
    return
  fi
  local dir; dir=$(mktemp -d)
  curl -fsSL "$LITESTREAM_URL" -o "$dir/litestream.deb"
  echo "$LITESTREAM_SHA256  $dir/litestream.deb" | sha256sum -c --quiet - || die "el .deb de Litestream no coincide con el SHA-256 fijado"
  # /etc/litestream.yml is a conffile of the package, and ours replaces it.
  # Without these options an upgrade stops to ask about it, behind the
  # >/dev/null. install_files puts ours back anyway.
  dpkg -i --force-confdef --force-confold "$dir/litestream.deb" >/dev/null
  rm -rf "$dir"
  # The package may start its own unit with its own config; ours replaces it below.
  note "$(litestream version)"
}

create_users() {
  log "Usuario quitsie y directorios"
  if ! id quitsie >/dev/null 2>&1; then
    useradd --system --home-dir /var/lib/quitsie --no-create-home --shell /usr/sbin/nologin quitsie
  fi
  install -d -m 0750 -o quitsie -g quitsie /var/lib/quitsie
  install -d -m 0750 -o root -g root "$ETC_DIR"
}

# read_value "Prompt" REGEX HIDDEN prints one validated value. Pasting the whole
# command the Cloudflare dashboard shows also works: only its last word counts.
read_value() {
  local prompt=$1 pattern=$2 hidden=$3 value=""
  [[ -t 0 ]] || die "necesito una terminal para pedir: $prompt (usa 'gcloud compute ssh ... -- -t')"
  while true; do
    if [[ $hidden == 1 ]]; then
      read -r -s -p "    $prompt: " value; echo >&2
    else
      read -r -p "    $prompt: " value
    fi
    value=${value%"${value##*[![:space:]]}"}   # trailing whitespace and CR
    value=${value##*[[:space:]]}               # keep the last word
    if [[ $value =~ $pattern ]]; then
      printf '%s' "$value"
      return
    fi
    echo "    Formato no válido; inténtalo de nuevo." >&2
  done
}

# has_value FILE KEY succeeds when FILE sets KEY to something non-empty.
has_value() { [[ -f $1 ]] && grep -Eq "^$2=.+" "$1"; }

# write_secret_file PATH LINE... replaces PATH atomically, root-only (mode 600).
write_secret_file() {
  local path=$1 tmp; shift
  tmp=$(mktemp "$path.XXXXXX")   # created 0600
  printf '%s\n' "$@" > "$tmp"
  chown root:root "$tmp"
  mv -f "$tmp" "$path"
}

write_secrets() {
  log "Secretos de la VM ($ETC_DIR)"
  local f=$ETC_DIR/litestream.env
  if has_value "$f" LITESTREAM_ACCESS_KEY_ID && has_value "$f" LITESTREAM_SECRET_ACCESS_KEY && has_value "$f" R2_ACCOUNT_ID; then
    note "$f ya tiene sus valores; no lo toco (para cambiarlo: sudoedit $f)"
  else
    echo "    Token de R2 (Cloudflare > R2 > Manage API tokens). No se muestran al escribirlos."
    local account key secret
    account=$(read_value "R2 Account ID (32 caracteres hexadecimales)" '^[0-9a-f]{32}$' 0)
    key=$(read_value "Access Key ID" '^[A-Za-z0-9]{16,64}$' 1)
    secret=$(read_value "Secret Access Key" '^[A-Za-z0-9+/=_-]{32,128}$' 1)
    write_secret_file "$f" "R2_ACCOUNT_ID=$account" "LITESTREAM_ACCESS_KEY_ID=$key" "LITESTREAM_SECRET_ACCESS_KEY=$secret"
    NEW_R2_KEYS=1
    note "escrito $f (modo 600)"
  fi

  f=$ETC_DIR/cloudflared.env
  if has_value "$f" TUNNEL_TOKEN; then
    note "$f ya tiene su valor; no lo toco (para cambiarlo: sudoedit $f)"
  else
    echo "    Token del túnel (Cloudflare Zero Trust > Networking > Tunnels > tu túnel)."
    local token
    token=$(read_value "Token del túnel" '^[A-Za-z0-9+/=_-]{80,}$' 1)
    write_secret_file "$f" "TUNNEL_TOKEN=$token"
    NEW_TUNNEL_TOKEN=1
    note "escrito $f (modo 600)"
  fi

  # The app's own values come after provisioning, because the Resend key needs
  # the domain verified first. The server does not start until they are in.
  if [[ ! -f $ETC_DIR/quitsie.env ]]; then
    install -m 0600 -o root -g root "$DEPLOY_DIR/quitsie.env.example" "$ETC_DIR/quitsie.env"
    note "creado $ETC_DIR/quitsie.env (modo 600), todo comentado: hay que rellenarlo antes del primer arranque"
  fi
}

# write_alert_email puts QUITSIE_ALERT_EMAIL in alert.env, where
# quitsie-alert-check reads it. Without it, the file keeps what it had.
write_alert_email() {
  log "Email de las alertas"
  local f=$ETC_DIR/alert.env email=${QUITSIE_ALERT_EMAIL:-}
  if [[ -n $email ]]; then
    [[ $email =~ $EMAIL_RE ]] || die "QUITSIE_ALERT_EMAIL no parece un email: $email"
    if grep -qxF "QUITSIE_ALERT_EMAIL=$email" "$f" 2>/dev/null; then
      note "las alertas ya van a $email"
    else
      write_secret_file "$f" "# Written by provision.sh. See deploy/alert.env.example." "QUITSIE_ALERT_EMAIL=$email"
      note "las alertas irán a $email ($f)"
    fi
  elif has_value "$f" QUITSIE_ALERT_EMAIL; then
    note "se conserva el email de $f"
  else
    if [[ ! -f $f ]]; then install -m 0600 -o root -g root "$DEPLOY_DIR/alert.env.example" "$f"; fi
    warn "sin email, las alertas solo quedan en el journal: vuelve a ejecutar con QUITSIE_ALERT_EMAIL=tu@email, o sudoedit $f"
  fi
}

# install_file SRC DEST MODE copies SRC to DEST when they differ.
install_file() {
  if ! cmp -s "$1" "$2"; then
    install -m "$3" -o root -g root "$1" "$2"
    note "instalado $2"
    return 0
  fi
  return 1
}

install_files() {
  log "Unidades systemd, Litestream y scripts"
  local unit
  for unit in "${UNITS[@]}"; do
    if install_file "$DEPLOY_DIR/$unit" "/etc/systemd/system/$unit" 0644; then CHANGED_UNITS=1; fi
  done
  for unit in "${WATCH_UNITS[@]}"; do
    if install_file "$DEPLOY_DIR/$unit" "/etc/systemd/system/$unit" 0644; then CHANGED_TIMERS=1; fi
  done
  if install_file "$DEPLOY_DIR/litestream.yml" /etc/litestream.yml 0644; then CHANGED_UNITS=1; fi
  install_file "$DEPLOY_DIR/quitsie-deploy.sh" /usr/local/sbin/quitsie-deploy 0755 || true
  install_file "$DEPLOY_DIR/restore-test.sh" /usr/local/sbin/quitsie-restore-test 0755 || true
  install_file "$DEPLOY_DIR/alert-check.sh" /usr/local/sbin/quitsie-alert-check 0755 || true
  systemctl daemon-reload
  # Informational: a missing /usr/local/bin/quitsie is expected before the first deploy.
  local paths=()
  for unit in "${UNITS[@]}" "${WATCH_UNITS[@]}"; do paths+=("/etc/systemd/system/$unit"); done
  systemd-analyze verify "${paths[@]}" 2>&1 | sed 's/^/    verify: /' || true
}

configure_deploy_user() {
  log "Usuario deploy (solo puede instalar el binario)"
  if ! id deploy >/dev/null 2>&1; then
    useradd --create-home --shell /bin/bash --comment 'Quitsie CI deploys' deploy
  fi
  # A password of "*" can never match, but unlike a locked account ("!") sshd
  # still lets the user in with a key.
  usermod -p '*' deploy

  local tmp
  tmp=$(mktemp)
  # No arguments allowed: the binary comes through stdin.
  printf 'deploy ALL=(root) NOPASSWD: /usr/local/sbin/quitsie-deploy ""\n' > "$tmp"
  visudo -cf "$tmp" >/dev/null || die "el archivo sudoers generado no es válido"
  install -m 0440 -o root -g root "$tmp" /etc/sudoers.d/quitsie-deploy
  rm -f "$tmp"

  local key=${QUITSIE_DEPLOY_PUBKEY:-} key_re='^ssh-ed25519 [A-Za-z0-9+/]+=*( [^[:cntrl:]]*)?$'
  local auth=/home/deploy/.ssh/authorized_keys
  if [[ -n $key ]]; then
    [[ $key =~ $key_re ]] || die "QUITSIE_DEPLOY_PUBKEY debe ser una sola línea 'ssh-ed25519 AAAA...'"
    install -d -m 0700 -o deploy -g deploy /home/deploy/.ssh
    tmp=$(mktemp)
    # restrict: no pty, no forwarding. The key can run the deploy script and nothing else.
    printf 'restrict,command="/usr/bin/sudo -n /usr/local/sbin/quitsie-deploy" %s\n' "$key" > "$tmp"
    install -m 0600 -o deploy -g deploy "$tmp" "$auth"
    rm -f "$tmp"
    note "clave de despliegue instalada en $auth"
  elif [[ -s $auth ]]; then
    note "se conserva la clave de despliegue que ya había"
  else
    warn "no hay clave de despliegue: vuelve a ejecutar con QUITSIE_DEPLOY_PUBKEY='ssh-ed25519 AAAA...'"
  fi
}

# start_unit UNIT [restart] starts UNIT, or restarts it when asked and it is running.
start_unit() {
  local unit=$1 action=start
  if [[ ${2:-} == restart ]] && systemctl is-active --quiet "$unit"; then action=restart; fi
  systemctl "$action" "$unit" || die "$unit no arrancó; mira: journalctl -u quitsie-restore -u $unit -n 30 --no-pager"
}

enable_services() {
  log "Servicios"
  systemctl enable cloudflared litestream quitsie-restore quitsie >/dev/null

  if [[ $NEW_TUNNEL_TOKEN == 1 ]]; then start_unit cloudflared restart; else start_unit cloudflared; fi

  # Starting litestream first runs quitsie-restore, which needs working R2 credentials.
  if [[ $CHANGED_UNITS == 1 || $NEW_R2_KEYS == 1 ]]; then start_unit litestream restart; else start_unit litestream; fi

  if [[ -x /usr/local/bin/quitsie ]]; then
    start_unit quitsie
  else
    note "aún no hay binario en /usr/local/bin/quitsie: el primer despliegue desde GitHub lo instala"
  fi

  local timer
  systemctl enable "${TIMERS[@]}" >/dev/null
  for timer in "${TIMERS[@]}"; do
    if [[ $CHANGED_TIMERS == 1 ]]; then start_unit "$timer" restart; else start_unit "$timer"; fi
  done
}

summary() {
  log "Estado"
  local unit
  for unit in cloudflared litestream quitsie-restore quitsie "${TIMERS[@]}"; do
    printf '    %-28s %s\n' "$unit" "$(systemctl is-active "$unit" || true)"
  done
  echo
  echo "    Para probar las alertas, cuando $ETC_DIR/quitsie.env tenga la clave de Resend:"
  echo "    sudo quitsie-alert-check --test"
  echo
  echo "    Clave pública de esta VM (no es secreta; va en la variable DEPLOY_SSH_HOST_KEY de GitHub):"
  echo "    $(awk '{print $1, $2}' /etc/ssh/ssh_host_ed25519_key.pub)"
  echo
  echo "    Siguiente: rellena $ETC_DIR/quitsie.env con sudoedit. Es obligatorio antes del primer arranque:"
  echo "    sin QUITSIE_PUBLIC_ORIGIN, QUITSIE_RESEND_API_KEY, QUITSIE_MAIL_FROM y QUITSIE_INVITE_SECRET el"
  echo "    servidor no arranca. Después publica la ruta del túnel y despliega (docs/despliegue.md)."
}

check_platform
install_packages
install_cloudflared
install_litestream
create_users
write_secrets
write_alert_email
install_files
configure_deploy_user
enable_services
summary
