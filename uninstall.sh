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
DEFAULT_PORT=8080

error() { echo "[错误] $1" >&2; exit 1; }
[ "$(id -u)" != "0" ] && error "请以 root 身份运行（sudo bash uninstall.sh）"

# ---- close firewall port opened by install.sh (best effort) ----
close_firewall_port() {
  local PORT="$1"
  [ -z "$PORT" ] && return 0

  if command -v firewall-cmd >/dev/null 2>&1 && firewall-cmd --state >/dev/null 2>&1; then
    firewall-cmd --permanent --remove-port="${PORT}/tcp" >/dev/null 2>&1 || true
    firewall-cmd --reload >/dev/null 2>&1 || true
    echo ">>> 已通过 firewalld 关闭端口 ${PORT}/tcp"
  elif command -v ufw >/dev/null 2>&1 && ufw status 2>/dev/null | grep -q "Status: active"; then
    ufw delete allow "${PORT}/tcp" >/dev/null 2>&1 || true
    echo ">>> 已通过 ufw 关闭端口 ${PORT}/tcp"
  elif command -v iptables >/dev/null 2>&1; then
    if iptables -D INPUT -p tcp --dport "${PORT}" -j ACCEPT >/dev/null 2>&1; then
      echo ">>> 已通过 iptables 关闭端口 ${PORT}/tcp"
    fi
  fi
}

cat <<'EOF'
 ____  ____                  _
|___ \|  _ \ __ _ _ __   ___| |
  __) | |_) / _` | '_ \ / _ \ |
 / __/|  __/ (_| | | | |  __/ |
|_____|_|   \__,_|_| |_|\___|_|

2Panel - 卸载
EOF

# ---- 1. stop & remove systemd service ----
PORT="$DEFAULT_PORT"
if command -v systemctl >/dev/null 2>&1 && [ -f "/etc/systemd/system/${SERVICE_NAME}.service" ]; then
  echo ">>> 正在停止并移除 systemd 服务 ${SERVICE_NAME} ..."
  PORT=$(grep -oP 'ExecStart=.*\K-port\s+\K[0-9]+' "/etc/systemd/system/${SERVICE_NAME}.service" 2>/dev/null | head -n1)
  [ -z "$PORT" ] && PORT="$DEFAULT_PORT"
  systemctl stop "${SERVICE_NAME}" 2>/dev/null || true
  systemctl disable "${SERVICE_NAME}" 2>/dev/null || true
  rm -f "/etc/systemd/system/${SERVICE_NAME}.service"
  systemctl daemon-reload 2>/dev/null || true
fi

# ---- 2. kill running process (background mode / fallback) ----
PIDS=""
if command -v pgrep >/dev/null 2>&1; then
  PIDS=$(pgrep -f "${BIN_PATH}" 2>/dev/null || true)
  [ -z "$PIDS" ] && PIDS=$(pgrep -x "2panel" 2>/dev/null || true)
fi
if [ -n "$PIDS" ]; then
  echo ">>> 正在停止 2panel 进程: $PIDS ..."
  for PID in $PIDS; do
    [ -d "/proc/$PID" ] || continue
    kill "$PID" 2>/dev/null || true
  done
  sleep 1
  # force kill if still alive
  for PID in $PIDS; do
    [ -d "/proc/$PID" ] || continue
    kill -9 "$PID" 2>/dev/null || true
  done
fi

# ---- 3. remove binary ----
if [ -f "${BIN_PATH}" ]; then
  rm -f "${BIN_PATH}"
  echo ">>> 已删除二进制文件 ${BIN_PATH}"
fi

# ---- 4. data directory (keep by default, it contains db/scripts/logs) ----
DATA_DIR="${DATA_DIR:-}"
[ -z "$DATA_DIR" ] && [ -d "${DEFAULT_DATA_DIR}" ] && DATA_DIR="${DEFAULT_DATA_DIR}"

if [ -n "$DATA_DIR" ] && [ -d "$DATA_DIR" ]; then
  read -r -p "是否删除数据目录 ${DATA_DIR}？（包含数据库、任务脚本和日志）[y/N]: " DEL_DATA
  case "$DEL_DATA" in
    y|Y|yes|YES)
      rm -rf "$DATA_DIR"
      echo ">>> 已删除数据目录 ${DATA_DIR}"
      ;;
    *)
      echo ">>> 已保留数据目录 ${DATA_DIR}"
      ;;
  esac
fi

# ---- 5. close firewall port ----
close_firewall_port "$PORT"

echo ""
echo "2Panel 已卸载完成。"
echo "如需同时删除本地源码目录，直接删除项目文件夹即可。"
