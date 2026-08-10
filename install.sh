#!/usr/bin/env bash
#
# 2Panel - 定时任务管理器
# One-line remote installer.
#
# Usage:
#   交互式安装:
#     bash -c "$(curl -sSL https://raw.githubusercontent.com/meimolihan/2Panel/main/install.sh)"
#   参数静默安装（-p 端口 / -d 数据目录）:
#     bash <(curl -sSL https://raw.githubusercontent.com/meimolihan/2Panel/main/install.sh) -p 8080 -d /var/lib/2panel
#     curl -fsSL https://raw.githubusercontent.com/meimolihan/2Panel/main/install.sh -o /tmp/2panel-install.sh \
#       && bash /tmp/2panel-install.sh -p 8080 -d /var/lib/2panel
#
# Requires a GitHub Release containing the binaries built by scripts/build-release.sh.

set -e

# ================== customize me ==================
GITHUB_OWNER="meimolihan"             # your GitHub username
GITHUB_REPO="2Panel"                  # GitHub repository name
DEFAULT_PORT=8080
DEFAULT_DATA_DIR="/var/lib/2panel"
# ==================================================

# ---- parse command-line args (silent install) ----
# 用法: bash install.sh [-p|--port PORT] [-d|--data DIR] [-h|--help]
# 示例: bash install.sh -p 8080 -d /var/lib/2panel
while [ "$#" -gt 0 ]; do
  case "$1" in
    -p|--port)
      shift
      [ -n "${1:-}" ] || { echo "缺少 -p/--port 的值" >&2; exit 1; }
      PORT="$1"
      ;;
    -d|--data)
      shift
      [ -n "${1:-}" ] || { echo "缺少 -d/--data 的值" >&2; exit 1; }
      DATA_DIR="$1"
      ;;
    -h|--help)
      echo "2Panel - 计划任务管理工具 安装脚本"
      echo "用法: bash install.sh [-p PORT] [-d DATA_DIR]"
      echo "  -p, --port   监听端口（默认 ${DEFAULT_PORT}）"
      echo "  -d, --data   数据目录（默认 ${DEFAULT_DATA_DIR}）"
      echo "  -h, --help   显示本帮助"
      echo "指定任意参数即进入静默安装；不带参数则为交互式安装。"
      exit 0
      ;;
    *)
      echo "未知参数: $1（使用 -h 查看帮助）" >&2
      exit 1
      ;;
  esac
  shift
done

API_BASE="https://api.github.com/repos/${GITHUB_OWNER}/${GITHUB_REPO}"

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

print_banner() {
  local z="$gl_zi" r="$reset" b="$gl_bai" l="$gl_lan"
  printf '%s\n' \
    "${z} ____  ____                  _${r}" \
    "${z}|___ \\|  _ \\ __ _ _ __   ___| |${r}" \
    "${z}  __) | |_) / _\` | '_ \\ / _ \\ |${r}" \
    "${z} / __/|  __/ (_| | | | |  __/ |${r}" \
    "${z}|_____|_|   \\__,_|_| |_|\\___|_|${r}" \
    "" \
    "${b}2Panel${r} - ${l}计划任务管理工具${r}" \
    ""
}

break_end() {
    echo -e "${gl_lv}操作完成${gl_bai}"
    if [ "${SILENT:-n}" = "y" ] || [ ! -t 0 ]; then
        echo ""
        return
    fi
    echo -e "${gl_bai}按任意键继续 ${gl_hong}.${gl_huang}.${gl_lv}.${gl_bai}\c"
    read -r -n 1 -s -r -p ""
    echo ""
    clear
}

error() { printf "  %s %s\n" "${gl_hong}[错误]${reset}" "$1" >&2; exit 1; }

# ---- firewall: automatically open the listen port ----
FW_OPENED="n"
open_firewall_port() {
  local PORT="$1"

  # 1. firewalld
  if command -v firewall-cmd >/dev/null 2>&1 && firewall-cmd --state >/dev/null 2>&1; then
    if ! firewall-cmd --query-port="${PORT}/tcp" >/dev/null 2>&1; then
      firewall-cmd --permanent --add-port="${PORT}/tcp" >/dev/null 2>&1 || true
      firewall-cmd --reload >/dev/null 2>&1 || true
    fi
    ok "已通过 ${gl_bai}firewalld${reset} 开放端口 ${gl_lan}${PORT}/tcp${reset}"
    FW_OPENED="y"
    return 0
  fi

  # 2. ufw
  if command -v ufw >/dev/null 2>&1 && ufw status 2>/dev/null | grep -q "Status: active"; then
    if ! ufw status 2>/dev/null | grep -q "${PORT}/tcp"; then
      ufw allow "${PORT}/tcp" >/dev/null 2>&1 || true
    fi
    ok "已通过 ${gl_bai}ufw${reset} 开放端口 ${gl_lan}${PORT}/tcp${reset}"
    FW_OPENED="y"
    return 0
  fi

  # 3. iptables
  if command -v iptables >/dev/null 2>&1; then
    if iptables -C INPUT -p tcp --dport "${PORT}" -j ACCEPT >/dev/null 2>&1; then
      ok "端口 ${gl_lan}${PORT}/tcp${reset} 已在 iptables 中放行"
      FW_OPENED="y"
      return 0
    fi
    if iptables -L INPUT -n 2>/dev/null | grep -qE 'policy (DROP|REJECT)|REJECT|DROP'; then
      if iptables -I INPUT -p tcp --dport "${PORT}" -j ACCEPT >/dev/null 2>&1; then
        ok "已通过 ${gl_bai}iptables${reset} 开放端口 ${gl_lan}${PORT}/tcp${reset}"
        FW_OPENED="y"
        return 0
      fi
    fi
  fi

  printf "  %s %s\n" "${gl_huang}[提示]${reset}" "未检测到活跃的防火墙（firewalld/ufw/iptables），跳过端口开放。"
}

# ---- preflight ----
[ "$(id -u)" != "0" ] && error "请以 root 身份运行（例如 sudo bash <(curl -sSL ...) -p 8080）"

command -v curl >/dev/null 2>&1 || error "请先安装 curl（apt install curl / yum install curl）"

ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64) ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *) error "不支持的架构: $ARCH（仅支持 amd64 / arm64）" ;;
esac

print_banner
sep_line
section "安装信息"
printf "  %-14s %s\n" "${gl_lan}系统${reset}" "$(uname -s) $(uname -m)"
printf "  %-14s %s\n" "${gl_lan}架构${reset}" "${ARCH}"
sep_line

# ---- silent install detection ----
# 通过命令行参数（-p/-d）或环境变量（PORT/DATA_DIR）跳过交互提示，实现静默安装：
#   bash install.sh -p 8080 -d /var/lib/2panel
#   PORT=8080 DATA_DIR=/var/lib/2panel bash install.sh
SILENT="n"
if [ -n "${PORT}" ]; then
  case "${PORT}" in
    ''|*[!0-9]*) error "环境变量 PORT 无效（需为 1-65535 的数字）: ${PORT}" ;;
    *) [ "${PORT}" -ge 1 ] && [ "${PORT}" -le 65535 ] || error "环境变量 PORT 超出范围（1-65535）: ${PORT}" ;;
  esac
  SILENT="y"
fi
if [ -n "${DATA_DIR}" ]; then
  SILENT="y"
fi

# ---- prompt: listen port ----
section "配置参数"
if [ -z "${PORT}" ]; then
  while :; do
    read -r -p "${gl_bai}请输入监听端口${reset} ${gl_hui}[默认: ${DEFAULT_PORT}]${reset}: " PORT
    PORT="${PORT:-$DEFAULT_PORT}"
    case "$PORT" in
      ''|*[!0-9]*) printf "  %s\n" "${gl_huang}端口无效，请重新输入。${reset}" ;;
      *)
        if [ "$PORT" -ge 1 ] && [ "$PORT" -le 65535 ]; then
          break
        fi
        printf "  %s\n" "${gl_huang}端口超出范围（1-65535），请重新输入。${reset}"
        ;;
    esac
  done
else
  printf "  %-14s %s\n" "${gl_lan}监听端口${reset}" "${gl_bai}${PORT}${reset}（参数指定）"
fi
PORT="${PORT:-$DEFAULT_PORT}"

# ---- prompt: data directory ----
if [ -z "${DATA_DIR}" ]; then
  read -r -p "${gl_bai}请输入数据目录${reset} ${gl_hui}[默认: ${DEFAULT_DATA_DIR}]${reset}: " DATA_DIR
else
  printf "  %-14s %s\n" "${gl_lan}数据目录${reset}" "${gl_bai}${DATA_DIR}${reset}（参数指定）"
fi
DATA_DIR="${DATA_DIR:-$DEFAULT_DATA_DIR}"

# ---- systemd: auto-configure (preferred) ----
if command -v systemctl >/dev/null 2>&1; then
  USE_SYSTEMD="y"
else
  USE_SYSTEMD="n"
  printf "  %s\n" "${gl_huang}[警告]${reset} 未检测到 systemd（容器或受限环境）。"
  printf "  %s\n" "    已回退为后台运行模式，重启或崩溃后服务不会自动恢复。"
fi

BIN_DIR="/usr/local/bin"
BIN_PATH="${BIN_DIR}/2panel"
BIN_URL="https://github.com/${GITHUB_OWNER}/${GITHUB_REPO}/releases/latest/download/2panel_linux_${ARCH}"
SHA_URL="${BIN_URL}.sha256"

sep_line
section "安装程序"
ok "正在下载 ${gl_bai}2panel${reset} (${gl_lan}${ARCH}${reset}) ..."
curl -fSL --retry 3 -o "${BIN_PATH}.download" "${BIN_URL}" || error "下载失败，请检查 GITHUB_OWNER 与 GitHub Release 附件"

# ---- verify checksum ----
if command -v sha256sum >/dev/null 2>&1; then
  if curl -fsSL --retry 2 -o "${BIN_PATH}.sha256" "${SHA_URL}" 2>/dev/null; then
    EXPECTED=$(awk '{print $1}' "${BIN_PATH}.sha256" | tr '[:upper:]' '[:lower:]')
    if [ -n "$EXPECTED" ]; then
      ACTUAL=$(sha256sum "${BIN_PATH}.download" | awk '{print $1}')
      if [ "$ACTUAL" != "$EXPECTED" ]; then
        error "下载文件校验失败（SHA-256 不匹配），已中止安装，请检查发布资产或网络是否被劫持"
      fi
      ok "SHA-256 校验通过"
    fi
  else
    printf "  %s %s\n" "${gl_huang}[警告]${reset}" "未找到 SHA-256 校验文件，已跳过完整性校验。"
    printf "  %s\n" "    发布资产中缺少 ${gl_lan}${ARCH}.sha256${reset}，或该文件下载失败。"
  fi
  rm -f "${BIN_PATH}.sha256"
fi

chmod +x "${BIN_PATH}.download"
mv "${BIN_PATH}.download" "${BIN_PATH}"

ok "已安装二进制至 ${gl_bai}${BIN_PATH}${reset}"
ok "正在创建数据目录 ${gl_lan}${DATA_DIR}${reset} ..."
mkdir -p "${DATA_DIR}"
chmod 700 "${DATA_DIR}"

# ---- write installation record ----
mkdir -p /etc/2panel
cat > /etc/2panel/config <<EOF
# 2Panel 安装记录（由 install.sh 生成，请勿手动修改）
BIN_PATH=${BIN_PATH}
PORT=${PORT}
DATA_DIR=${DATA_DIR}
EOF
chmod 0644 /etc/2panel/config
ok "已写入安装记录 ${gl_bai}/etc/2panel/config${reset}"

"${BIN_PATH}" -version

sep_line
section "启动服务"
if [ "$USE_SYSTEMD" = "y" ]; then
  cat > /etc/systemd/system/2panel.service <<UNIT
[Unit]
Description=2Panel - 计划任务管理工具
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
    ok "2panel 服务已启动。"
    systemctl status 2panel --no-pager || true
  else
    printf "  %s\n" "${gl_hong}[错误]${reset} 服务启动失败，请检查：${gl_bai}journalctl -u 2panel -n 50${reset}" >&2
    exit 1
  fi
else
  if command -v pgrep >/dev/null 2>&1 && pgrep -x "2panel" >/dev/null 2>&1; then
    printf "  %s\n" "${gl_huang}[警告]${reset} 检测到 2panel 进程可能已在运行"
  else
    nohup "${BIN_PATH}" -port "${PORT}" -data "${DATA_DIR}" >> "${DATA_DIR}/2panel.log" 2>&1 &
    ok "2panel 已在后台启动，pid: ${gl_bai}$!${reset}"
  fi
fi

IP=$(hostname -I 2>/dev/null | awk '{print $1}')
[ -z "$IP" ] && IP="<服务器IP>"

open_firewall_port "$PORT"

if [ "$FW_OPENED" = "y" ]; then
  FW_STATUS="${gl_lv}已开放 ${PORT}/tcp${reset}"
else
  FW_STATUS="${gl_huang}未检测到活跃防火墙，已跳过${reset}"
fi

sep_line
if [ "$USE_SYSTEMD" = "y" ]; then
  printf "  %s\n" "${gl_lv}✔ 2Panel 安装成功！${reset}"
  printf "  %-14s %s\n" "${gl_lan}访问地址${reset}" "${gl_bai}http://${IP}:${PORT}${reset}"
  printf "  %-14s %s\n" "${gl_lan}数据目录${reset}" "${gl_bai}${DATA_DIR}${reset}"
  printf "  %-14s %s\n" "${gl_lan}二进制文件${reset}" "${gl_bai}${BIN_PATH}${reset}"
  printf "  %-14s %s\n" "${gl_lan}防火墙状态${reset}" "$FW_STATUS"
  printf "  %-14s %s\n" "${gl_lan}运行模式${reset}" "${gl_bai}systemd 服务${reset}"
  printf "  %-14s %s\n" "${gl_lan}服务命令${reset}" "${gl_hui}systemctl status 2panel${reset}"
else
  printf "  %s\n" "${gl_lv}✔ 2Panel 安装成功！${reset} ${gl_huang}（后台运行模式）${reset}"
  printf "  %-14s %s\n" "${gl_lan}访问地址${reset}" "${gl_bai}http://${IP}:${PORT}${reset}"
  printf "  %-14s %s\n" "${gl_lan}数据目录${reset}" "${gl_bai}${DATA_DIR}${reset}"
  printf "  %-14s %s\n" "${gl_lan}二进制文件${reset}" "${gl_bai}${BIN_PATH}${reset}"
  printf "  %-14s %s\n" "${gl_lan}防火墙状态${reset}" "$FW_STATUS"
  printf "  %-14s %s\n" "${gl_lan}运行模式${reset}" "${gl_huang}后台运行${reset}"
  printf "  %s\n" "  ${gl_huang}注意：${reset}后台运行模式在系统重启后不会自动恢复。"
  printf "  %s\n" "    如需开机自启 / 崩溃自动重启 / journald 日志，请在安装 systemd 后重新运行本脚本。"
fi
sep_line
break_end
