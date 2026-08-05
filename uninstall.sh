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

# ---- 0. confirm uninstall (service & program) ----
while :; do
  read -r -p "卸载将停止并移除 2Panel 服务与程序，是否继续？[y/N]: " CONFIRM
  case "$CONFIRM" in
    y|Y|yes|YES)
      echo ">>> 开始卸载 2Panel ..."
      break
      ;;
    n|N|no|NO|"")
      echo "已取消卸载。"
      exit 0
      ;;
    *)
      echo "  输入无效，请输入 y 或 n。"
      ;;
  esac
done

# ---- 1. stop & remove systemd service ----
# 先从服务文件提取实际使用的端口与数据目录（卸载时一并清理）
PORT="$DEFAULT_PORT"
DATA_DIR="${DATA_DIR:-}"
if command -v systemctl >/dev/null 2>&1 && [ -f "/etc/systemd/system/${SERVICE_NAME}.service" ]; then
  echo ">>> 正在停止并移除 systemd 服务 ${SERVICE_NAME} ..."
  SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"
  PORT=$(grep -oE '\-port [0-9]+' "$SERVICE_FILE" | awk '{print $2}' | head -n1)
  [ -z "$PORT" ] && PORT="$DEFAULT_PORT"
  [ -z "$DATA_DIR" ] && DATA_DIR=$(grep -oE '\-data [^ ]+' "$SERVICE_FILE" | awk '{print $2}' | head -n1)
  systemctl stop "${SERVICE_NAME}" 2>/dev/null || true
  systemctl disable "${SERVICE_NAME}" 2>/dev/null || true
  rm -f "$SERVICE_FILE"
  systemctl daemon-reload 2>/dev/null || true
fi

# ---- 2. kill running process (background mode / fallback) ----
PIDS=""
if command -v pgrep >/dev/null 2>&1; then
  PIDS=$(pgrep -f "${BIN_PATH}" 2>/dev/null || true)
  [ -z "$PIDS" ] && PIDS=$(pgrep -x "2panel" 2>/dev/null || true)
fi
if [ -n "$PIDS" ]; then
  # 无 systemd 场景：从运行进程 cmdline 解析实际端口与数据目录（供后续清理）
  if [ -z "$DATA_DIR" ]; then
    for PID in $PIDS; do
      [ -d "/proc/$PID" ] || continue
      CMD=$(tr '\0' ' ' < "/proc/$PID/cmdline" 2>/dev/null)
      [ -z "$PORT_CMD" ] && PORT_CMD=$(printf '%s' "$CMD" | grep -oE '\-port [0-9]+' | awk '{print $2}')
      [ -z "$DATA_DIR" ] && DATA_DIR=$(printf '%s' "$CMD" | grep -oE '\-data [^ ]+' | awk '{print $2}')
      [ -n "$PORT_CMD" ] && [ -n "$DATA_DIR" ] && break
    done
  fi
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

# 优先级：systemd 服务文件 > 运行进程 cmdline > 环境变量 > 默认目录
[ -z "$PORT" ] && [ -n "$PORT_CMD" ] && PORT="$PORT_CMD"
[ -n "$DATA_DIR" ] && DEFAULT_DATA_DIR="$DATA_DIR"

# ---- 3. remove binary ----
if [ -f "${BIN_PATH}" ]; then
  rm -f "${BIN_PATH}"
  echo ">>> 已删除二进制文件 ${BIN_PATH}"
fi

# ---- 4. data directory (persist data: db/scripts/logs) ----
# DATA_DIR 已按优先级解析：systemd 服务文件 > 运行进程 cmdline > 环境变量 > 默认目录。
# 删除前确认，默认删除（回车=删除，输入 n 保留）。
[ -z "$DATA_DIR" ] && [ -d "${DEFAULT_DATA_DIR}" ] && DATA_DIR="${DEFAULT_DATA_DIR}"

if [ -n "$DATA_DIR" ] && [ -d "$DATA_DIR" ]; then
  echo ">>> 检测到数据目录: ${DATA_DIR}"
  read -r -p "是否删除数据目录 ${DATA_DIR}？（包含数据库、任务脚本和日志）[Y/n]: " DEL_DATA
  case "$DEL_DATA" in
    n|N|no|NO)
      echo ">>> 已保留数据目录 ${DATA_DIR}"
      ;;
    *)
      rm -rf "$DATA_DIR"
      echo ">>> 已删除数据目录 ${DATA_DIR}"
      ;;
  esac
fi

# ---- 5. close firewall port ----
close_firewall_port "$PORT"

echo ""
echo "2Panel 已卸载完成。如需重新安装，请再次运行 install.sh 安装脚本。"
