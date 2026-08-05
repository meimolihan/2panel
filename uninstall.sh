#!/usr/bin/env bash
#
# 2Panel - uninstall script.
# Stops the service/process, removes the binary and (optionally) the data
# directory created by install.sh.
#
# Usage: bash uninstall.sh

set -e

BIN_PATH="/usr/local/bin/2panel"
SERVICE_NAME="2panel"
DEFAULT_DATA_DIR="/var/lib/2panel"

error() { echo "[ERROR] $1" >&2; exit 1; }
[ "$(id -u)" != "0" ] && error "please run as root (sudo bash uninstall.sh)"

cat <<'EOF'
  ______ _____
 |___  / /  _ \
    / / | |_) |
   / /  |  __/
  /_/   |_|

  2Panel - Uninstall
EOF

# ---- 1. stop & remove systemd service ----
if command -v systemctl >/dev/null 2>&1 && [ -f "/etc/systemd/system/${SERVICE_NAME}.service" ]; then
  echo ">>> stopping and removing systemd service ${SERVICE_NAME} ..."
  systemctl stop "${SERVICE_NAME}" 2>/dev/null || true
  systemctl disable "${SERVICE_NAME}" 2>/dev/null || true
  rm -f "/etc/systemd/system/${SERVICE_NAME}.service"
  systemctl daemon-reload 2>/dev/null || true
fi

# ---- 2. kill running process (background mode / fallback) ----
PIDS=""
if command -v pgrep >/dev/null 2>&1; then
  PIDS=$(pgrep -f "^${BIN_PATH}" 2>/dev/null || true)
  [ -z "$PIDS" ] && PIDS=$(pgrep -x "2panel" 2>/dev/null || true)
fi
if [ -n "$PIDS" ]; then
  echo ">>> stopping running 2panel process(es): $PIDS ..."
  # shellcheck disable=SC2086
  kill $PIDS 2>/dev/null || true
  sleep 1
  # force kill if still alive
  # shellcheck disable=SC2086
  kill -9 $PIDS 2>/dev/null || true
fi

# ---- 3. remove binary ----
if [ -f "${BIN_PATH}" ]; then
  rm -f "${BIN_PATH}"
  echo ">>> removed binary ${BIN_PATH}"
fi

# ---- 4. data directory (keep by default, it contains db/scripts/logs) ----
DATA_DIR="${DATA_DIR:-}"
[ -z "$DATA_DIR" ] && [ -d "${DEFAULT_DATA_DIR}" ] && DATA_DIR="${DEFAULT_DATA_DIR}"

if [ -n "$DATA_DIR" ] && [ -d "$DATA_DIR" ]; then
  read -r -p "Remove data directory ${DATA_DIR} ? (contains database, task scripts and logs) [y/N]: " DEL_DATA
  case "$DEL_DATA" in
    y|Y|yes|YES)
      rm -rf "$DATA_DIR"
      echo ">>> removed data directory ${DATA_DIR}"
      ;;
    *)
      echo ">>> kept data directory ${DATA_DIR}"
      ;;
  esac
fi

echo ""
echo "2Panel has been uninstalled."
echo "If you also want to remove the local source checkout, just delete the project folder."
