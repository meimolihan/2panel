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
CONFIG_FILE="/etc/2panel/config"

list_color_init() {
    export gl_hui=$'\033[38;5;59m'
    export gl_hong=$'\033[38;5;9m'
    export gl_lv=$'\033[38;5;10m'
    export gl_huang=$'\033[38;5;11m'
    export gl_lan=$'\033[38;5;32m'
    export gl_bai=$'\033[38;5;15m'
    export gl_zi=$'\033[38;5;13m'
    export gl_bufan=$'\033[38;5;14m'
    export reset=$'\033[0m'
}
list_color_init

sep_line() {
  printf '%s' "$gl_bufan"
  printf '—%.0s' {1..32}
  printf '%s\n' "$reset"
}

section() {
  printf "  %s %s\n" "${gl_zi}▶${reset}" "$1"
}

ok() {
  printf "  %s %s\n" "${gl_lv}>>>${reset}" "$1"
}

skip() {
  printf "  %s %s\n" "${gl_hui}---${reset}" "$1"
}

print_banner() {
  local z="$gl_zi" r="$reset" b="$gl_bai" l="$gl_lan"
  printf '%s\n' \
    "${z} ____  ____                  _${r}" \
    "${z}|___ \\|  _ \\ __ _ _ __   ___| |${r}" \
    "${z}  __) | |_) / _\` | '_ \\ / _ \\ |${r}" \
    "${z} / __/|  __/ (_| | | | |  __/ |${r}" \
    "${z}|_____|_|   \\__,_|_| |_|\\___|_|${r}" \
    ""
}

break_end() {
    echo -e "${gl_lv}操作完成${gl_bai}"
    echo -e "${gl_bai}按任意键继续 ${gl_hong}.${gl_huang}.${gl_lv}.${gl_bai}\c"
    read -r -n 1 -s -r -p ""
    echo ""
    clear
}

error() { printf "  %s %s\n" "${gl_hong}[错误]${reset}" "$1" >&2; exit 1; }
[ "$(id -u)" != "0" ] && error "请以 root 身份运行（sudo bash uninstall.sh）"

read_config() {
  [ -f "$CONFIG_FILE" ] || return 0
  while IFS='=' read -r KEY VALUE; do
    KEY=$(printf '%s' "$KEY" | tr -d ' ')
    VALUE=$(printf '%s' "$VALUE" | tr -d '\r')
    case "$KEY" in
      BIN_PATH) [ -n "$VALUE" ] && BIN_PATH="$VALUE" ;;
      PORT) [ -n "$VALUE" ] && PORT="$VALUE" ;;
      DATA_DIR) [ -n "$VALUE" ] && DATA_DIR="$VALUE" ;;
    esac
  done < "$CONFIG_FILE"
}

find_2panel_pids() {
  local d pid exe
  for d in /proc/[0-9]*; do
    [ -d "$d" ] || continue
    pid="${d#/proc/}"
    [ "$pid" = "$$" ] && continue
    exe=$(readlink "$d/exe" 2>/dev/null) || continue
    [ "$(basename "$exe")" = "2panel" ] || continue
    echo "$pid"
  done
}

close_firewall_port() {
  local PORT="$1"
  [ -z "$PORT" ] && return 0

  # 1. firewalld
  if command -v firewall-cmd >/dev/null 2>&1 && firewall-cmd --state >/dev/null 2>&1; then
    firewall-cmd --permanent --remove-port="${PORT}/tcp" >/dev/null 2>&1 || true
    firewall-cmd --reload >/dev/null 2>&1 || true
    ok "已通过 ${gl_bai}firewalld${reset} 关闭端口 ${gl_lan}${PORT}/tcp${reset}"
  # 2. ufw
  elif command -v ufw >/dev/null 2>&1 && ufw status 2>/dev/null | grep -q "Status: active"; then
    ufw delete allow "${PORT}/tcp" >/dev/null 2>&1 || true
    ok "已通过 ${gl_bai}ufw${reset} 关闭端口 ${gl_lan}${PORT}/tcp${reset}"
  # 3. iptables
  elif command -v iptables >/dev/null 2>&1; then
    if iptables -D INPUT -p tcp --dport "${PORT}" -j ACCEPT >/dev/null 2>&1; then
      ok "已通过 ${gl_bai}iptables${reset} 关闭端口 ${gl_lan}${PORT}/tcp${reset}"
    fi
  fi
}

print_banner
echo -e "${gl_zi}>>> 卸载 2Panel${gl_bai}"
sep_line
while :; do
  read -r -p "${gl_bai}卸载将停止并移除 2Panel 服务与程序，是否继续？ (${gl_lv}y${gl_bai}/${gl_hong}N${gl_bai}): " CONFIRM
  case "$CONFIRM" in
    y|Y|yes|YES)
      ok "开始卸载 2Panel ${gl_hong}.${gl_huang}.${gl_lv}.${gl_bai}"
      break
      ;;
    n|N|no|NO|"")
      printf "  %s\n" "${gl_huang}已取消卸载。${reset}"
      exit 0
      ;;
    *)
      printf "  %s\n" "${gl_huang}输入无效，请输入 y 或 n。${reset}"
      ;;
  esac
done

PORT="$DEFAULT_PORT"
DATA_DIR="${DATA_DIR:-}"
read_config

echo -e ""
if command -v systemctl >/dev/null 2>&1 && [ -f "/etc/systemd/system/${SERVICE_NAME}.service" ]; then
  SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"
  [ -z "$PORT" ] && PORT=$(grep -oE '\-port [0-9]+' "$SERVICE_FILE" | awk '{print $2}' | head -n1)
  [ -z "$PORT" ] && PORT="$DEFAULT_PORT"
  [ -z "$DATA_DIR" ] && DATA_DIR=$(grep -oE '\-data [^ ]+' "$SERVICE_FILE" | awk '{print $2}' | head -n1)
  ok "正在停止并移除 systemd 服务 ${gl_bai}${SERVICE_NAME}${reset} ${gl_hong}.${gl_huang}.${gl_lv}.${gl_bai}"
  systemctl stop "${SERVICE_NAME}" 2>/dev/null || true
  systemctl disable "${SERVICE_NAME}" 2>/dev/null || true
  rm -f "$SERVICE_FILE"
  systemctl daemon-reload 2>/dev/null || true
else
  skip "未发现 systemd 服务，跳过。"
fi

echo -e ""
PIDS=$(find_2panel_pids)
if [ -n "$PIDS" ]; then
  if [ -z "$DATA_DIR" ]; then
    for PID in $PIDS; do
      [ -d "/proc/$PID" ] || continue
      CMD=$(tr '\0' ' ' < "/proc/$PID/cmdline" 2>/dev/null)
      [ -z "$PORT_CMD" ] && PORT_CMD=$(printf '%s' "$CMD" | grep -oE '\-port [0-9]+' | awk '{print $2}')
      [ -z "$DATA_DIR" ] && DATA_DIR=$(printf '%s' "$CMD" | grep -oE '\-data [^ ]+' | awk '{print $2}')
      [ -n "$PORT_CMD" ] && [ -n "$DATA_DIR" ] && break
    done
  fi
  ok "正在停止 2panel 进程: ${gl_bai}$PIDS${reset} ${gl_hong}.${gl_huang}.${gl_lv}.${gl_bai}"
  for PID in $PIDS; do
    [ -d "/proc/$PID" ] || continue
    kill "$PID" 2>/dev/null || true
  done
  sleep 1
  for PID in $PIDS; do
    [ -d "/proc/$PID" ] || continue
    kill -9 "$PID" 2>/dev/null || true
  done
else
  skip "未发现运行中的 2panel 进程，跳过。"
fi

[ -z "$PORT" ] && [ -n "$PORT_CMD" ] && PORT="$PORT_CMD"
[ -n "$DATA_DIR" ] && DEFAULT_DATA_DIR="$DATA_DIR"

echo -e ""
if [ -f "${BIN_PATH}" ]; then
  rm -f "${BIN_PATH}"
  ok "已删除二进制文件 ${gl_bai}${BIN_PATH}${reset}"
else
  skip "未找到二进制文件 ${gl_bai}${BIN_PATH}${reset}，跳过。"
fi

echo -e ""
[ -z "$DATA_DIR" ] && [ -d "${DEFAULT_DATA_DIR}" ] && DATA_DIR="${DEFAULT_DATA_DIR}"

if [ -n "$DATA_DIR" ] && [ -d "$DATA_DIR" ]; then
  ok "检测到数据目录: ${gl_bai}${DATA_DIR}${reset}"
  read -r -p "${gl_huang}是否删除数据目录 ${DATA_DIR}？（包含数据库、任务脚本和日志）${gl_bai}[Y/n]${reset}: " DEL_DATA
  case "$DEL_DATA" in
    n|N|no|NO)
      skip "已保留数据目录 ${gl_bai}${DATA_DIR}${reset}"
      ;;
    *)
      rm -rf "$DATA_DIR"
      ok "已删除数据目录 ${gl_bai}${DATA_DIR}${reset}"
      ;;
  esac
else
  skip "未找到数据目录，跳过。"
fi

echo -e ""
if [ -f "$CONFIG_FILE" ]; then
  rm -f "$CONFIG_FILE"
  ok "已删除安装记录 ${gl_bai}$CONFIG_FILE${reset}"
  rmdir "$(dirname "$CONFIG_FILE")" 2>/dev/null || true
else
  skip "未找到安装记录 ${gl_bai}$CONFIG_FILE${reset}，跳过。"
fi

echo -e ""
close_firewall_port "$PORT"

echo -e ""
printf "  %s\n" "${gl_lv}✔ 2Panel 已卸载完成${reset}"
printf "  %s\n" "${gl_hui}如需重新安装，请再次运行 install.sh 安装脚本。${reset}"
sep_line
