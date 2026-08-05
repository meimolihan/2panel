#!/usr/bin/env bash
#
# 2Panel - Scheduled Task Manager
# One-line remote installer.
#
# Usage:
#   bash -c "$(curl -sSL https://raw.githubusercontent.com/<OWNER>/2Panel/main/install.sh)"
#
# Before publishing, replace GITHUB_OWNER below with your GitHub username and
# create a GitHub Release containing the binaries built by scripts/build-release.sh.

set -e

# ================== customize me ==================
GITHUB_OWNER="meimolihan"   # TODO: your GitHub username
GITHUB_REPO="2Panel"                  # GitHub repository name
DEFAULT_PORT=8080
DEFAULT_DATA_DIR="/var/lib/2panel"
# ==================================================

API_BASE="https://api.github.com/repos/${GITHUB_OWNER}/${GITHUB_REPO}"

print_banner() {
  cat <<'EOF'
  ______ _____
 |___  / /  _ \
    / / | |_) |
   / /  |  __/
  /_/   |_|

  2Panel - Scheduled Task Manager
EOF
}

error() { echo "[ERROR] $1" >&2; exit 1; }

# ---- preflight ----
[ "$(id -u)" != "0" ] && error "please run as root (e.g. sudo bash -c \"$(curl -sSL ...)\")"

command -v curl >/dev/null 2>&1 || error "curl is required, please install it first (apt install curl / yum install curl)"

ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64) ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *) error "unsupported architecture: $ARCH (only amd64 / arm64 are supported)" ;;
esac

print_banner
echo "============================================================"
echo " Installing 2Panel"
echo "   OS   : $(uname -s) $(uname -m)"
echo "   Arch : $ARCH"
echo "============================================================"

# ---- prompt: listen port ----
while :; do
  read -r -p "Enter listen port [default: ${DEFAULT_PORT}]: " PORT
  PORT="${PORT:-$DEFAULT_PORT}"
  case "$PORT" in
    ''|*[!0-9]*) echo "  invalid port number, please retry." ;;
    *)
      if [ "$PORT" -ge 1 ] && [ "$PORT" -le 65535 ]; then
        break
      fi
      echo "  port out of range (1-65535), please retry."
      ;;
  esac
done

# ---- prompt: data directory ----
read -r -p "Enter data directory [default: ${DEFAULT_DATA_DIR}]: " DATA_DIR
DATA_DIR="${DATA_DIR:-$DEFAULT_DATA_DIR}"

# ---- systemd: auto-configure (preferred) ----
# systemd gives auto-start on boot, crash restart and unified journald logs.
# Fall back to background mode only when systemd is unavailable (containers).
if command -v systemctl >/dev/null 2>&1; then
  USE_SYSTEMD="y"
else
  USE_SYSTEMD="n"
  echo ""
  echo "[WARN] systemd not found (container / limited environment)."
  echo "       Falling back to background mode - the service will NOT"
  echo "       auto-restart after a reboot or crash."
fi

BIN_DIR="/usr/local/bin"
BIN_PATH="${BIN_DIR}/2panel"
BIN_URL="https://github.com/${GITHUB_OWNER}/${GITHUB_REPO}/releases/latest/download/2panel_linux_${ARCH}"

echo ""
echo ">>> downloading 2panel (${ARCH}) ..."
curl -fSL --retry 3 -o "${BIN_PATH}.download" "${BIN_URL}" || error "download failed, please check GITHUB_OWNER and the GitHub Release assets"
chmod +x "${BIN_PATH}.download"
mv "${BIN_PATH}.download" "${BIN_PATH}"

echo ">>> creating data directory ${DATA_DIR} ..."
mkdir -p "${DATA_DIR}"
chmod 700 "${DATA_DIR}"

"${BIN_PATH}" -version

if [ "$USE_SYSTEMD" = "y" ]; then
  cat > /etc/systemd/system/2panel.service <<UNIT
[Unit]
Description=2Panel - Scheduled Task Manager
After=network.target

[Service]
Type=simple
ExecStart=${BIN_PATH} -port ${PORT} -data ${DATA_DIR}
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
UNIT

  systemctl daemon-reload
  systemctl enable 2panel >/dev/null 2>&1 || true
  systemctl restart 2panel || true
  sleep 2
  if systemctl is-active 2panel >/dev/null 2>&1; then
    echo ">>> 2panel service started."
    systemctl status 2panel --no-pager || true
  else
    echo "[ERROR] service failed to start, check: journalctl -u 2panel -n 50" >&2
    exit 1
  fi
else
  if command -v pgrep >/dev/null 2>&1 && pgrep -f "${BIN_PATH} -port ${PORT}" >/dev/null 2>&1; then
    echo "[WARN] 2panel may already be running on port ${PORT}"
  else
    nohup "${BIN_PATH}" -port "${PORT}" -data "${DATA_DIR}" >> "${DATA_DIR}/2panel.log" 2>&1 &
    echo ">>> 2panel started in background, pid: $!"
  fi
fi

IP=$(hostname -I 2>/dev/null | awk '{print $1}')
[ -z "$IP" ] && IP="<server-ip>"

if [ "$USE_SYSTEMD" = "y" ]; then
  cat <<EOF

============================================================
  2Panel has been installed successfully!

    Web UI   : http://${IP}:${PORT}
    Data dir : ${DATA_DIR}
    Binary   : ${BIN_PATH}
    Version  : ${BIN_PATH} -version

  Registered as systemd service "2panel":
    systemctl status 2panel      # check status
    systemctl restart 2panel     # restart
    journalctl -u 2panel -f      # follow logs
    systemctl disable 2panel     # stop auto-start on boot
============================================================
EOF
else
  cat <<EOF

============================================================
  2Panel has been installed successfully! (background mode)

    Web UI   : http://${IP}:${PORT}
    Data dir : ${DATA_DIR}
    Binary   : ${BIN_PATH}
    Version  : ${BIN_PATH} -version

  NOTE: running in background, it will not restart on reboot.
        Install systemd on this host and re-run this script to
        get auto-start / crash-restart / journald logs.
============================================================
EOF
fi
