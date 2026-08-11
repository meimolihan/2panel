#!/usr/bin/env bash
#
# 2Panel - uninstall script.
# Stops the service/process, removes the binary and (optionally) the data
# directory created by install.sh.
#
# Usage: bash uninstall.sh
#

set -euo pipefail

BIN_PATH="/usr/local/bin/2panel"
SERVICE_NAME="2panel"
DEFAULT_DATA_DIR="/var/lib/2panel"
DEFAULT_PORT=8080
CONFIG_FILE="/etc/2panel/config"

list_color_init() {
    # 非TTY关闭颜色
    if [ ! -t 1 ]; then
        export gl_hui=""
        export gl_hong=""
        export gl_lv=""
        export gl_huang=""
        export gl_lan=""
        export gl_bai=""
        export gl_zi=""
        export gl_bufan=""
        export reset=""
        return
    fi
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

section() {
  printf "  %s %s\n" "${gl_zi}▶${reset}" "$1"
}

ok() {
  printf "  %s %s\n" "${gl_lv}>>>${reset}" "$1"
}

skip() {
  printf "  %s %s\n" "${gl_hui}----${reset}" "$1"
}

warn() {
  printf "  %s %s\n" "${gl_huang}[提示]${reset}" "$1"
}

error() {
  printf "  %s %s\n" "${gl_hong}[错误]${reset}" "$1" >&2
  exit 1
}

print_banner() {
  local z="$gl_zi" r="$reset"
  cat <<EOF
${z} ____  ____                  _${r}
${z}|___ \\|  _ \\ __ _ _ __   ___| |${r}
${z}  __) | |_) / _\` | '_ \\ / _ \\ |${r}
${z} / __/|  __/ (_| | | | |  __/ |${r}
${z}|_____|_|   \\__,_|_| |_|\\___|_|${r}
EOF
  echo ""
}

[ "$(id -u)" != "0" ] && error "请以 root 身份运行（sudo bash uninstall.sh）"

# 读取配置，跳过注释行
read_config() {
  local key_val
  [ -f "$CONFIG_FILE" ] || return 0
  while IFS='=' read -r key_val; do
    # 跳过空行、#注释
    [[ -z "$key_val" || "$key_val" =~ ^# ]] && continue
    local KEY VALUE
    KEY=$(printf '%s' "$key_val" | cut -d'=' -f1 | tr -d ' ')
    VALUE=$(printf '%s' "$key_val" | cut -d'=' -f2 | tr -d '\r')
    case "$KEY" in
      BIN_PATH) [ -n "$VALUE" ] && BIN_PATH="$VALUE" ;;
      PORT) [ -n "$VALUE" ] && PORT="$VALUE" ;;
      DATA_DIR) [ -n "$VALUE" ] && DATA_DIR="$VALUE" ;;
    esac
  done < "$CONFIG_FILE"
}

# 获取所有2panel pid，排除僵尸进程
find_2panel_pids() {
  local d pid exe stat
  for d in /proc/[0-9]*; do
    [ -d "$d" ] || continue
    pid="${d#/proc/}"
    [ "$pid" = "$$" ] && continue
    stat=$(cat "${d}/stat" 2>/dev/null || true)
    # 跳过僵尸进程 Z
    if [[ "$stat" =~ \(.*\)\ Z ]]; then
        continue
    fi
    exe=$(readlink "${d}/exe" 2>/dev/null || true)
    [ "$(basename "$exe")" = "2panel" ] || continue
    echo "$pid"
  done
}

# 从pid的cmdline解析 -port / -data
parse_from_pid() {
  local pid="$1"
  local cmdline
  cmdline=$(tr '\0' ' ' < "/proc/${pid}/cmdline" 2>/dev/null || true)
  local p d
  p=$(printf '%s' "$cmdline" | grep -oE '\-port [0-9]+' | awk '{print $2}' || true)
  d=$(printf '%s' "$cmdline" | grep -oE '\-data [^ ]+' | awk '{print $2}' || true)
  echo "${p:-} ${d:-}"
}

close_firewall_port() {
  local PORT="$1"
  [ -z "$PORT" ] && return 0

  # 只清理本脚本之前开放的端口，不盲目删除全部规则
  if command -v firewall-cmd >/dev/null 2>&1 && firewall-cmd --state >/dev/null 2>&1; then
    if firewall-cmd --query-port="${PORT}/tcp" >/dev/null 2>&1; then
      firewall-cmd --permanent --remove-port="${PORT}/tcp" >/dev/null 2>&1 || true
      firewall-cmd --reload >/dev/null 2>&1 || true
      ok "已通过 firewalld 移除端口 ${PORT}/tcp"
    else
      skip "firewalld 中未发现 ${PORT}/tcp，跳过"
    fi
    return 0
  fi

  if command -v ufw >/dev/null 2>&1 && ufw status 2>/dev/null | grep -q "Status: active"; then
    if ufw status 2>/dev/null | grep -q "${PORT}/tcp"; then
      ufw delete allow "${PORT}/tcp" >/dev/null 2>&1 || true
      ok "已通过 ufw 移除端口 ${PORT}/tcp"
    else
      skip "ufw 中未发现 ${PORT}/tcp，跳过"
    fi
    return 0
  fi

  if command -v iptables >/dev/null 2>&1; then
    if iptables -C INPUT -p tcp --dport "${PORT}" -j ACCEPT >/dev/null 2>&1; then
      iptables -D INPUT -p tcp --dport "${PORT}" -j ACCEPT >/dev/null 2>&1
      ok "已通过 iptables 移除端口 ${PORT}/tcp"
    else
      skip "iptables INPUT链未发现 ${PORT}/tcp 放行规则，跳过"
    fi
  fi
}

# -------- main --------
print_banner
printf "%s>>> %s\n" "${gl_zi}" "${gl_bai}卸载 2Panel${reset}"
printf "%s%s\n" "${gl_bufan}" "————————————————————————————————————————————————${reset}"

CONFIRM="n"
# TTY交互式确认；非TTY直接执行卸载（静默模式）
if [ -t 0 ]; then
  while :; do
    read -r -p "${gl_bai}卸载将停止并移除 2Panel 服务与程序，是否继续？ (${gl_lv}y${gl_bai}/${gl_hong}N${gl_bai}): " CONFIRM
    case "$CONFIRM" in
      y|Y|yes|YES)
        ok "开始卸载 2Panel"
        break
        ;;
      n|N|no|NO|"")
        warn "已取消卸载。"
        exit 0
        ;;
      *)
        warn "输入无效，请输入 y 或 n。"
        ;;
    esac
  done
else
  warn "非交互式环境，自动确认卸载"
  CONFIRM="y"
fi

PORT="$DEFAULT_PORT"
DATA_DIR=""
read_config

section "停止 systemd 服务"
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"
if command -v systemctl >/dev/null 2>&1 && [ -f "$SERVICE_FILE" ]; then
  # 从service文件回填参数
  port_svc=$(grep -oE '\-port [0-9]+' "$SERVICE_FILE" | awk '{print $2}' | head -n1 || true)
  data_svc=$(grep -oE '\-data [^ ]+' "$SERVICE_FILE" | awk '{print $2}' | head -n1 || true)
  [ -n "$port_svc" ] && PORT="$port_svc"
  [ -n "$data_svc" ] && DATA_DIR="$data_svc"

  systemctl stop "${SERVICE_NAME}" 2>/dev/null || true
  systemctl disable "${SERVICE_NAME}" 2>/dev/null || true
  rm -f "$SERVICE_FILE"
  systemctl daemon-reload 2>/dev/null || true
  ok "已停止、禁用并删除 systemd unit"
else
  skip "未发现 systemd 服务单元，跳过"
fi

section "终止运行进程"
PIDS=$(find_2panel_pids)
if [ -n "$PIDS" ]; then
  ok "发现运行PID: ${PIDS}"
  # 从第一个运行进程尝试回填PORT/DATA_DIR
  read p_from_pid d_from_pid <<< $(parse_from_pid $(echo "$PIDS" | awk '{print $1}'))
  [ -n "$p_from_pid" ] && PORT="$p_from_pid"
  [ -n "$d_from_pid" ] && DATA_DIR="$d_from_pid"

  # 优雅终止
  for PID in $PIDS; do
    [ -d "/proc/$PID" ] && kill "$PID" 2>/dev/null || true
  done
  sleep 1
  # 强制杀残留
  for PID in $PIDS; do
    if [ -d "/proc/$PID" ]; then
      warn "进程 $PID 未退出，执行强制终止 SIGKILL"
      kill -9 "$PID" 2>/dev/null || true
    fi
  done
  ok "所有2panel进程已终止"
else
  skip "未发现运行中的2panel进程，跳过"
fi

section "删除二进制程序"
if [ -f "${BIN_PATH}" ]; then
  rm -f "${BIN_PATH}"
  ok "已删除二进制 ${BIN_PATH}"
else
  skip "二进制文件不存在 ${BIN_PATH}，跳过"
fi

section "处理数据目录"
# 兜底默认路径
[ -z "$DATA_DIR" ] && DATA_DIR="$DEFAULT_DATA_DIR"
# 安全防护：禁止高危路径rm‑rf
DANGER_PATHS=( "/" "/etc" "/usr" "/bin" "/sbin" "/usr/bin" "/usr/sbin" "/var" )
is_danger=0
for dp in "${DANGER_PATHS[@]}"; do
    if [ "$DATA_DIR" = "$dp" ]; then
        is_danger=1
        break
    fi
done

if [ "$is_danger" -eq 1 ]; then
  warn "检测到高危路径，跳过数据目录删除: ${DATA_DIR}"
elif [ -n "$DATA_DIR" ] && [ -d "$DATA_DIR" ]; then
  DEL_DATA="n"
  if [ -t 0 ]; then
    read -r -p "${gl_bai}是否删除数据目录 ${gl_huang}${DATA_DIR}${gl_bai}？（数据库/任务/日志）(${gl_lv}y${gl_bai}/${gl_hong}N${gl_bai}): " DEL_DATA
  else
    DEL_DATA="n"
    warn "非交互式，保留数据目录"
  fi

  case "$DEL_DATA" in
    y|Y|yes|YES)
      rm -rf "${DATA_DIR}"
      ok "已删除数据目录 ${DATA_DIR}"
      ;;
    *)
      skip "保留数据目录 ${DATA_DIR}"
      ;;
  esac
else
  skip "数据目录不存在，跳过"
fi

section "清理配置记录"
if [ -f "$CONFIG_FILE" ]; then
  rm -f "$CONFIG_FILE"
  ok "删除安装记录文件 ${CONFIG_FILE}"
  # 删除空目录，有残留文件不会报错
  rmdir "$(dirname "$CONFIG_FILE")" 2>/dev/null || true
else
  skip "未找到安装记录，跳过"
fi

section "关闭防火墙端口"
close_firewall_port "${PORT}"

printf "\n%s%s\n" "${gl_bufan}" "————————————————————————————————————————————————${reset}"
printf "  %s\n" "${gl_lv}✔ 2Panel 卸载完成${reset}"
printf "  %‑16s %s\n" "二进制" "${BIN_PATH}"
printf "  %‑16s %s\n" "监听端口" "${PORT}"
printf "  %‑16s %s\n" "数据目录" "${DATA_DIR}"
printf "\n"