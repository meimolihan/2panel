#!/usr/bin/env bash
#
# 2Panel - 定时任务管理器 卸载脚本
# 停止并移除 systemd 服务 / 后台进程，删除二进制与安装记录，可选删除数据目录。
#
# Usage: bash scripts/uninstall.sh [-y] [--purge|--keep-data] [-q]

set -e

APP_NAME="2panel"
BIN_PATH="/usr/local/bin/${APP_NAME}"
DEFAULT_DATA_DIR="/var/lib/${APP_NAME}"
DEFAULT_PORT=8080
RECORD_FILE="/etc/${APP_NAME}.conf"
LEGACY_RECORD_FILE="/etc/${APP_NAME}/config"
SERVICE_FILE="/etc/systemd/system/${APP_NAME}.service"

# ================== terminal colors ==================
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
  printf "  %s %s\n" "${gl_hui}--${reset}" "$1"
}

print_banner() {
  local z="$gl_zi" r="$reset" b="$gl_bai" l="$gl_lan"
  printf '%s\n' \
    "${z} ____  ____                  _${r}" \
    "${z}|___ \\|  _ \\ __ _ _ __   ___| |${r}" \
    "${z}  __) | |_) / _\` | '_ \\ / _ \\ |${r}" \
    "${z} / __/|  __/ (_| | | | |  __/ |${r}" \
    "${z}|_____|_|   \\__,_|_| |_|\\___|_|${r}" \
    "" \
    "${b}2Panel${r} - ${l}卸载${r}" \
    ""
}

error() { printf "  %s %s\n" "${gl_hong}[错误]${reset}" "$1" >&2; exit 1; }
[ "$(id -u)" != "0" ] && error "请以 root 身份运行（sudo bash scripts/uninstall.sh）"

UNINSTALL_YES=0
DELETE_DATA=0
KEEP_DATA=0
QUIET=0

usage() {
  printf '%s\n' \
    "用法: bash scripts/uninstall.sh [选项]" \
    "" \
    "选项:" \
    "  -y, --yes        免确认，自动同意卸载" \
    "      --purge      卸载时同时删除数据目录（包含数据库、任务脚本和日志）" \
    "      --keep-data  卸载时保留数据目录" \
    "  -q, --quiet      静默模式，仅输出关键信息" \
    "  -h, --help       显示帮助" \
    "" \
    "示例:" \
    "  bash scripts/uninstall.sh -y               免确认卸载，保留数据目录" \
    "  bash scripts/uninstall.sh -y --purge       免确认卸载，并删除数据目录"
  exit 0
}

# ---- bootstrap: support `bash -c "$(curl ...)" -y --purge` ----
# In `bash -c "script" args` the first arg becomes $0, so a flag passed right
# after the script string would be invisible to the normal $1.. parsing below.
# Re-prepend it when $0 looks like a flag (the `_` placeholder form keeps working).
case "$0" in
  -*) set -- "$0" "$@" ;;
esac

while [ $# -gt 0 ]; do
  case "$1" in
    -y|--yes) UNINSTALL_YES=1; shift ;;
    --purge|--delete-data) DELETE_DATA=1; shift ;;
    --keep-data) KEEP_DATA=1; shift ;;
    -q|--quiet) QUIET=1; shift ;;
    -h|--help) usage ;;
    *) error "未知参数: $1，使用 -h 查看帮助" ;;
  esac
done

[ "$QUIET" = "1" ] && {
  sep_line() { :; }
  section() { :; }
  ok() { :; }
  skip() { :; }
}

# read_config 读取安装记录：优先新版 /etc/2panel.conf，回退旧版 /etc/2panel/config
read_config() {
  local f=""
  for f in "${RECORD_FILE}" "${LEGACY_RECORD_FILE}"; do
    [ -f "$f" ] || continue
    while IFS='=' read -r KEY VALUE; do
      KEY=$(printf '%s' "$KEY" | tr -d ' ')
      VALUE=$(printf '%s' "$VALUE" | tr -d '\r')
      case "$KEY" in
        BIN_PATH) [ -n "$VALUE" ] && BIN_PATH="$VALUE" ;;
        PORT) [ -n "$VALUE" ] && PORT="$VALUE" ;;
        DATA_DIR) [ -n "$VALUE" ] && DATA_DIR="$VALUE" ;;
      esac
    done < "$f"
    return 0
  done
  return 0
}

find_app_pids() {
  local d pid exe
  for d in /proc/[0-9]*; do
    [ -d "$d" ] || continue
    pid="${d#/proc/}"
    [ "$pid" = "$$" ] && continue
    exe=$(readlink "$d/exe" 2>/dev/null) || continue
    [ "$(basename "$exe")" = "${APP_NAME}" ] || continue
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

[ "$QUIET" = "1" ] || print_banner
sep_line
section "卸载确认"
if [ "$UNINSTALL_YES" = "1" ]; then
  ok "开始卸载 ${APP_NAME} ${gl_hong}.${gl_huang}.${gl_lv}.${reset}"
else
  while :; do
    read -r -p "${gl_huang}卸载将停止并移除 ${APP_NAME} 服务与程序，是否继续？${gl_bai}[y/N]${reset}: " CONFIRM
    case "$CONFIRM" in
      y|Y|yes|YES)
        ok "开始卸载 ${APP_NAME} ${gl_hong}.${gl_huang}.${gl_lv}.${reset}"
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
fi

PORT="$DEFAULT_PORT"
DATA_DIR=""
read_config

# 从 systemd 服务文件回退读取安装参数（安装记录缺失时）
if [ -f "$SERVICE_FILE" ]; then
  [ -z "$PORT" ] && PORT=$(grep -oE '\-port [0-9]+' "$SERVICE_FILE" | awk '{print $2}' | head -n1)
  [ -z "$PORT" ] && PORT="$DEFAULT_PORT"
  [ -z "$DATA_DIR" ] && DATA_DIR=$(grep -oE '\-data [^ ]+' "$SERVICE_FILE" | awk '{print $2}' | head -n1)
fi

# 从运行中进程命令行回退读取（非 systemd 安装）
if [ -z "$DATA_DIR" ] || [ -z "$PORT" ]; then
  for PID in $(find_app_pids); do
    [ -d "/proc/$PID" ] || continue
    CMD=$(tr '\0' ' ' < "/proc/$PID/cmdline" 2>/dev/null)
    [ -z "$PORT" ] && PORT=$(printf '%s' "$CMD" | grep -oE '\-port [0-9]+' | awk '{print $2}' | head -n1)
    [ -z "$DATA_DIR" ] && DATA_DIR=$(printf '%s' "$CMD" | grep -oE '\-data [^ ]+' | awk '{print $2}' | head -n1)
    [ -n "$PORT" ] && [ -n "$DATA_DIR" ] && break
  done
fi
[ -z "$PORT" ] && PORT="$DEFAULT_PORT"
[ -z "$DATA_DIR" ] && DATA_DIR="$DEFAULT_DATA_DIR"

sep_line
section "停止服务"
if command -v systemctl >/dev/null 2>&1 && [ -f "$SERVICE_FILE" ]; then
  ok "正在停止并移除 systemd 服务 ${gl_bai}${APP_NAME}${reset} ..."
  systemctl stop "${APP_NAME}" 2>/dev/null || true
  systemctl disable "${APP_NAME}" 2>/dev/null || true
  systemctl reset-failed "${APP_NAME}" 2>/dev/null || true
  rm -f "$SERVICE_FILE"
  systemctl daemon-reload 2>/dev/null || true
else
  skip "未发现 systemd 服务，跳过。"
fi

sep_line
section "停止进程"
PIDS=$(find_app_pids)
if [ -n "$PIDS" ]; then
  ok "正在停止 ${APP_NAME} 进程: ${gl_bai}$PIDS${reset} ..."
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
  skip "未发现运行中的 ${APP_NAME} 进程，跳过。"
fi

sep_line
section "删除二进制"
if [ -f "${BIN_PATH}" ]; then
  rm -f "${BIN_PATH}"
  ok "已删除二进制文件 ${gl_bai}${BIN_PATH}${reset}"
else
  skip "未找到二进制文件 ${gl_bai}${BIN_PATH}${reset}，跳过。"
fi

sep_line
section "删除数据目录"
if [ -n "$DATA_DIR" ] && [ -d "$DATA_DIR" ]; then
  ok "检测到数据目录: ${gl_bai}${DATA_DIR}${reset}"
  if [ "$KEEP_DATA" = "1" ]; then
    skip "已保留数据目录 ${gl_bai}${DATA_DIR}${reset}"
  elif [ "$DELETE_DATA" = "1" ]; then
    rm -rf "$DATA_DIR"
    ok "已删除数据目录 ${gl_bai}${DATA_DIR}${reset}"
  elif [ -t 0 ]; then
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
    skip "非交互模式下默认保留数据目录 ${gl_bai}${DATA_DIR}${reset}"
  fi
else
  skip "未找到数据目录，跳过。"
fi

sep_line
section "删除安装记录"
REMOVED_RECORD="n"
for f in "${RECORD_FILE}" "${LEGACY_RECORD_FILE}"; do
  if [ -f "$f" ]; then
    rm -f "$f"
    ok "已删除安装记录 ${gl_bai}$f${reset}"
    REMOVED_RECORD="y"
  else
    skip "未找到安装记录 ${gl_bai}$f${reset}，跳过。"
  fi
done
if [ "$REMOVED_RECORD" = "y" ]; then
  rmdir "$(dirname "${LEGACY_RECORD_FILE}")" 2>/dev/null || true
fi

sep_line
section "关闭防火墙"
close_firewall_port "$PORT"

sep_line
printf "  %s\n" "${gl_lv}✔ ${APP_NAME} 已卸载完成${reset}"
printf "  %s\n" "${gl_hui}如需重新安装，请再次运行 scripts/install.sh 安装脚本。${reset}"
sep_line