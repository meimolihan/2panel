#!/usr/bin/env bash
#
# 2Panel - 定时任务管理器
# 将本地产物（go build -o 2panel . 或 dist/2panel_linux_<arch>）安装为 systemd
# 服务；本地无构建产物时自动从 GitHub Release 下载对应架构二进制。
# 可重复执行，升级等同于重新安装（覆盖二进制，按上一次安装记录预填参数并重启服务）。
#
# Usage:
#   交互式安装（将提示端口与数据目录）:
#     bash scripts/install.sh
#   参数静默安装（-p 端口 / -d 数据目录 / -b 二进制源 / -y 免交互）:
#     bash scripts/install.sh -p 8080 -d /var/lib/2panel
#     bash -c "$(curl -sSL https://raw.githubusercontent.com/meimolihan/2Panel/main/scripts/install.sh)" -p 8080 -d /var/lib/2panel
#     curl -fsSL https://raw.githubusercontent.com/meimolihan/2Panel/main/scripts/install.sh -o /tmp/2panel-install.sh \
#       && bash /tmp/2panel-install.sh -p 8080 -d /var/lib/2panel
#
# Requires a GitHub Release containing the binaries built by scripts/build-release.sh.

set -euo pipefail

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
    "${b}2Panel${r} - ${l}计划任务管理工具 · 安装${r}" \
    ""
}

error() { printf "  %s %s\n" "${gl_hong}[错误]${reset}" "$1" >&2; exit 1; }

# ================== customize me ==================
APP_NAME="2panel"
GITHUB_OWNER="meimolihan"
GITHUB_REPO="2Panel"
DEFAULT_PORT=8080
DEFAULT_DATA_DIR="/var/lib/${APP_NAME}"
BIN_PATH="/usr/local/bin/${APP_NAME}"
RECORD_FILE="/etc/${APP_NAME}.conf"
SERVICE_FILE="/etc/systemd/system/${APP_NAME}.service"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-}")" && pwd)"
DEFAULT_BIN_SRC="${SCRIPT_DIR}/../${APP_NAME}"
# ==================================================

# ================== GitHub 下载加速镜像 ==================
# 原始 GitHub 地址超时/失败时，按下列顺序依次尝试（末尾必须带斜杠）
GITHUB_MIRRORS=(
  "https://ghfast.top/"
  "https://ghproxy.net/"
  "https://gh.xxooo.cf/"
  "https://v6.gh-proxy.org/"
  "https://githubproxy.cc/"
)
# v6.gh-proxy.org 为纯 IPv6 代理：本机未配置 IPv6 地址时剔除，避免空等超时
if [ ! -s /proc/net/if_inet6 ]; then
  _no_v6=()
  for _m in "${GITHUB_MIRRORS[@]}"; do
    case "${_m}" in
      *v6.gh-proxy.org*) continue ;;
    esac
    _no_v6+=("${_m}")
  done
  GITHUB_MIRRORS=("${_no_v6[@]}")
fi

# 原始 GitHub URL -> 候选地址列表（原始优先，再依次套用各镜像）
make_url_candidates() {
  local github_url="$1" p
  printf '%s\n' "${github_url}"
  for p in "${GITHUB_MIRRORS[@]}"; do
    printf '%s\n' "${p}${github_url}"
  done
}

# 下载单个文件：候选按序尝试，单链接单次 120s 超时后换源。
# 用法: download_file <URL> <输出文件> [期望魔数hex]
#   魔数为可选，用于甄别镜像返回的错误页/截断文件（7f454c46=ELF、1f8b=gzip）。
#   下载成功后尝试拉取 <URL>.sha256 校验（校验文件缺失时跳过，魔数兜底）。
# 任一候选成功返回 0；全部失败返回 1。
download_file() {
  local url="$1" dst="$2" want="${3:-}" u="" hex="" esha="" ahex=""
  local sha_file="${dst}.sha256"
  while IFS= read -r u; do
    printf "  %s\n" "${gl_hui}--${reset} 尝试下载 ${gl_bai}${u}${reset}"
    rm -f "${dst}"
    if command -v curl >/dev/null 2>&1; then
      if command -v timeout >/dev/null 2>&1; then
        timeout 120 curl -fsSL --connect-timeout 10 --max-time 120 -o "${dst}" "${u}" 2>/dev/null || continue
      else
        curl -fsSL --connect-timeout 10 --max-time 120 -o "${dst}" "${u}" 2>/dev/null || continue
      fi
    elif command -v wget >/dev/null 2>&1; then
      wget -qO "${dst}" --timeout=120 --tries=1 "${u}" 2>/dev/null || continue
    else
      return 1
    fi
    [ -s "${dst}" ] || continue
    if [ -n "${want}" ]; then
      hex="$(head -c 4 "${dst}" | od -An -tx1 | tr -d ' \n')"
      case "${hex}" in
        "${want}"*) ;;
        *) printf "  %s\n" "${gl_huang}[警告]${reset} 下载内容非预期(${u})，换源重试。" >&2; continue ;;
      esac
    fi
    # sha256 校验（校验文件缺失时跳过，仅依赖魔数兜底）
    if command -v sha256sum >/dev/null 2>&1; then
      rm -f "${sha_file}"
      if { command -v curl >/dev/null 2>&1 && curl -fsSL --connect-timeout 10 --max-time 60 -o "${sha_file}" "${u}.sha256" 2>/dev/null; } || \
         { command -v wget >/dev/null 2>&1 && wget -qO "${sha_file}" --timeout=60 --tries=1 "${u}.sha256" 2>/dev/null; }; then
        if [ -s "${sha_file}" ]; then
          esha="$(awk '{print $1}' "${sha_file}" | tr '[:upper:]' '[:lower:]')"
          if [ -n "${esha}" ]; then
            ahex="$(sha256sum "${dst}" | awk '{print $1}')"
            if [ "${ahex}" != "${esha}" ]; then
              printf "  %s\n" "${gl_huang}[警告]${reset} SHA-256 校验失败(${u})，换源重试。" >&2
              continue
            fi
            ok "SHA‑256 校验通过"
          fi
        fi
      fi
    fi
    return 0
  done < <(make_url_candidates "${url}")
  return 1
}
# ==================================================

# 经 curl|bash 远程执行时，SCRIPT_DIR 指向 bash 抽取的临时目录，本地产物需按
# 常见目录回退探测（仓库根目录 / dist / bin / 上一级目录），否则会误判"无本地产物"。
resolve_local_src() {
  local rel_arch="" c
  case "$(uname -m)" in
    x86_64|amd64) rel_arch="amd64" ;;
    aarch64|arm64) rel_arch="arm64" ;;
  esac
  local candidates=(
    "${SCRIPT_DIR:-}/../bin/${APP_NAME}"
    "${SCRIPT_DIR:-}/../${APP_NAME}"
    "${SCRIPT_DIR:-}/../dist/${APP_NAME}_linux_${rel_arch}"
    "$(pwd)/bin/${APP_NAME}"
    "$(pwd)/${APP_NAME}"
    "$(pwd)/dist/${APP_NAME}_linux_${rel_arch}"
    "$(pwd)/../${APP_NAME}"
    "$(dirname "$(pwd)")/${APP_NAME}"
  )
  for c in "${candidates[@]}"; do
    [ -n "${c}" ] || continue
    if [ -s "${c}" ]; then
      printf '%s' "${c}"
      return 0
    fi
  done
  return 1
}
# ==================================================

PORT=""
DATA_DIR=""
BIN_SRC=""
BIN_SRC_EXPLICIT=0
INSTALL_YES=0

# ---- bootstrap: support `bash -c "$(curl ...)" -p ... -d ...` ----
# In `bash -c "script" args` the first arg becomes $0, so a flag passed right
# after the script string would be invisible to the normal $1.. parsing below.
# Re-prepend it when $0 looks like a flag (the `_` placeholder form keeps working).
case "$0" in
  -*) set -- "$0" "$@" ;;
esac

# ---- parse command-line args (silent install) ----
while [ "$#" -gt 0 ]; do
  case "$1" in
    -p|--port)
      shift
      [ -n "${1:-}" ] || error "缺少 -p/--port 的值"
      PORT="$1"
      ;;
    -d|--data)
      shift
      [ -n "${1:-}" ] || error "缺少 -d/--data 的值"
      DATA_DIR="$1"
      ;;
    -b|--bin)
      shift
      [ -n "${1:-}" ] || error "缺少 -b/--bin 的值"
      BIN_SRC="$1"
      BIN_SRC_EXPLICIT=1
      ;;
    -y|--yes)
      INSTALL_YES=1
      ;;
    -h|--help)
      printf "%s\n" "${gl_lan}2Panel${reset} - ${gl_bai}计划任务管理工具 安装脚本${reset}"
      printf "  %-13s %s\n" "${gl_bai}用法:${reset}" "bash scripts/install.sh [-p PORT] [-d DATA_DIR] [-b BIN] [-y]"
      printf "  %-13s %s\n" "${gl_bai}-p, --port${reset}" "监听端口（默认 ${gl_lan}${DEFAULT_PORT}${reset}）"
      printf "  %-13s %s\n" "${gl_bai}-d, --data${reset}" "数据目录（默认 ${gl_lan}${DEFAULT_DATA_DIR}${reset}）"
      printf "  %-13s %s\n" "${gl_bai}-b, --bin${reset}" "二进制源路径（默认自动探测仓库根目录 / dist / bin 下的本地产物）"
      printf "  %-13s %s\n" "${gl_bai}-y, --yes${reset}" "免交互，未指定项全部使用默认值"
      printf "  %-13s %s\n" "${gl_bai}-h, --help${reset}" "显示本帮助"
      printf "%s\n" "${gl_hui}指定任意参数即进入静默安装；不带参数则为交互式安装。${reset}"
      printf "%s\n" "${gl_hui}未指定 -b 且本地无构建产物时，自动从 GitHub Release 下载对应架构二进制。${reset}"
      exit 0
      ;;
    *)
      error "未知参数: $1（使用 -h 查看帮助）"
      ;;
  esac
  shift
done

# ---- read previous install record to prefill defaults (reinstall/upgrade) ----
read_record() {
  [ -f "${RECORD_FILE}" ] || return 0
  while IFS='=' read -r KEY VALUE; do
    KEY=$(printf '%s' "$KEY" | tr -d ' ')
    VALUE=$(printf '%s' "$VALUE" | tr -d '\r')
    case "$KEY" in
      BIN_PATH) [ -n "$VALUE" ] && BIN_PATH="$VALUE" ;;
      PORT) [ -n "$VALUE" ] && [ -z "$PORT" ] && PORT="$VALUE" ;;
      DATA_DIR) [ -n "$VALUE" ] && [ -z "$DATA_DIR" ] && DATA_DIR="$VALUE" ;;
    esac
  done < "${RECORD_FILE}"
  return 0
}

# ---- firewall: automatically open the listen port ----
FW_OPENED="n"
open_firewall_port() {
  local PORT="$1"
  if command -v firewall-cmd >/dev/null 2>&1 && firewall-cmd --state >/dev/null 2>&1; then
    if ! firewall-cmd --query-port="${PORT}/tcp" >/dev/null 2>&1; then
      firewall-cmd --permanent --add-port="${PORT}/tcp" >/dev/null 2>&1 || true
      firewall-cmd --reload >/dev/null 2>&1 || true
    fi
    ok "已通过 ${gl_bai}firewalld${reset} 开放端口 ${gl_lan}${PORT}/tcp${reset}"
    FW_OPENED="y"
    return 0
  fi

  if command -v ufw >/dev/null 2>&1 && ufw status 2>/dev/null | grep -q "Status: active"; then
    if ! ufw status 2>/dev/null | grep -q "${PORT}/tcp"; then
      ufw allow "${PORT}/tcp" >/dev/null 2>&1 || true
    fi
    ok "已通过 ${gl_bai}ufw${reset} 开放端口 ${gl_lan}${PORT}/tcp${reset}"
    FW_OPENED="y"
    return 0
  fi

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

[ "$(id -u)" != "0" ] && error "请以 root 身份运行（例如 sudo bash scripts/install.sh）"
command -v curl >/dev/null 2>&1 || command -v wget >/dev/null 2>&1 || \
  error "请先安装 curl 或 wget（apt install curl / yum install curl）"

read_record

ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64) ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
esac

print_banner
sep_line
section "安装信息"
printf "  %-14s %s\n" "${gl_lan}系统${reset}" "$(uname -s) $(uname -m)"
printf "  %-14s %s\n" "${gl_lan}架构${reset}" "${ARCH}"
printf "  %-14s %s\n" "${gl_lan}程序${reset}" "${gl_bai}${APP_NAME}${reset}"
sep_line

# ---- silent install detection ----
if [ -n "${PORT}" ]; then
  case "${PORT}" in
    ''|*[!0-9]*) error "PORT 无效（需为 1‑65535 的数字）: ${PORT}" ;;
    *) [ "${PORT}" -ge 1 ] && [ "${PORT}" -le 65535 ] || error "PORT 超出范围（1‑65535）: ${PORT}" ;;
  esac
fi
if [ "$INSTALL_YES" = "1" ] || [ -n "${PORT}" ] || [ -n "${DATA_DIR}" ] || [ -n "${BIN_SRC}" ] || [ ! -t 0 ]; then
  SILENT="y"
else
  SILENT="n"
fi

section "配置参数"
# port prompt
if [ -z "${PORT}" ]; then
  if [ "$INSTALL_YES" = "1" ] || [ ! -t 0 ]; then
    PORT="${DEFAULT_PORT}"
  else
    while :; do
      read -r -p "${gl_bai}请输入监听端口${reset} ${gl_hui}[默认: ${DEFAULT_PORT}]${reset}: " PORT
      PORT="${PORT:-$DEFAULT_PORT}"
      case "$PORT" in
        ''|*[!0-9]*) printf "  %s\n" "${gl_huang}端口无效，请重新输入。${reset}" ;;
        *)
          if [ "$PORT" -ge 1 ] && [ "$PORT" -le 65535 ]; then break; fi
          printf "  %s\n" "${gl_huang}端口超出范围（1‑65535），请重新输入。${reset}"
          ;;
      esac
    done
  fi
else
  printf "  %-14s %s\n" "${gl_lan}监听端口${reset}" "${gl_bai}${PORT}${reset}（参数指定）"
fi
PORT="${PORT:-$DEFAULT_PORT}"

# data dir prompt
if [ -z "${DATA_DIR}" ]; then
  if [ "$INSTALL_YES" = "1" ] || [ ! -t 0 ]; then
    DATA_DIR="${DEFAULT_DATA_DIR}"
  else
    read -r -p "${gl_bai}请输入数据目录${reset} ${gl_hui}[默认: ${DEFAULT_DATA_DIR}]${reset}: " DATA_DIR
    DATA_DIR="${DATA_DIR:-$DEFAULT_DATA_DIR}"
  fi
else
  printf "  %-14s %s\n" "${gl_lan}数据目录${reset}" "${gl_bai}${DATA_DIR}${reset}（参数指定）"
fi
DATA_DIR="${DATA_DIR:-$DEFAULT_DATA_DIR}"

# binary source
BIN_SRC="${BIN_SRC:-$DEFAULT_BIN_SRC}"
if [ ! -s "${BIN_SRC}" ]; then
  # 经 curl|bash 远程执行：探测当前目录/仓库目录中的本地产物
  if [ "${BIN_SRC_EXPLICIT}" != "1" ]; then
    DISCOVERED_BIN="$(resolve_local_src)" || true
    if [ -n "${DISCOVERED_BIN:-}" ]; then
      ok "已从仓库目录发现本地产物 ${gl_bai}${DISCOVERED_BIN}${reset}"
      BIN_SRC="${DISCOVERED_BIN}"
    fi
  fi
fi
if [ ! -s "${BIN_SRC}" ]; then
  if [ "${BIN_SRC_EXPLICIT}" = "1" ]; then
    error "未找到二进制文件 ${BIN_SRC}（-b 显式指定）"
  fi
  # 本地无构建产物时，尝试从 GitHub Release 下载指定架构的静态二进制
  case "${ARCH}" in
    amd64|arm64) REL_ARCH="${ARCH}" ;;
    *) error "不支持的架构: $(uname -m)，请先本地构建（go build -o ${APP_NAME} .）或使用 -b 指定" ;;
  esac
  REL_URL="https://github.com/${GITHUB_OWNER}/${GITHUB_REPO}/releases/latest/download/${APP_NAME}_linux_${REL_ARCH}"
  ok "本地无构建产物，尝试从 GitHub Release 下载 ${gl_bai}${REL_URL}${reset}"
  TMP_BIN="$(mktemp)"
  if ! download_file "${REL_URL}" "${TMP_BIN}" "7f454c46"; then
    rm -f "${TMP_BIN}"
    error "下载 Release 二进制失败（${REL_URL}），请先本地构建（go build -o ${APP_NAME} .）或使用 -b 指定"
  fi
  chmod +x "${TMP_BIN}"
  BIN_SRC="${TMP_BIN}"
  ok "已从 GitHub Release 下载二进制（${gl_bai}$(du -h "${TMP_BIN}" | cut -f1)${reset}）"
fi

if command -v systemctl >/dev/null 2>&1; then
  USE_SYSTEMD="y"
else
  USE_SYSTEMD="n"
  printf "  %s\n" "${gl_huang}[警告]${reset} 未检测到 systemd（容器或受限环境）。"
  printf "  %s\n" "${gl_hui}    已回退为后台运行模式，重启或崩溃后服务不会自动恢复。${reset}"
fi

sep_line
section "安装程序"
ok "正在安装 ${gl_bai}${APP_NAME}${reset} 二进制 ${gl_hong}.${gl_huang}.${gl_lv}.${gl_bai}"

cp -f "${BIN_SRC}" "${BIN_PATH}"
chmod +x "${BIN_PATH}"
ok "已安装二进制至 ${gl_bai}${BIN_PATH}${reset}"

ok "正在创建数据目录 ${gl_lan}${DATA_DIR}${reset}"
mkdir -p "${DATA_DIR}"
chmod 700 "${DATA_DIR}"

# ---- write install record ----
mkdir -p "$(dirname "${RECORD_FILE}")"
cat > "${RECORD_FILE}" <<EOF
# ${APP_NAME} 安装记录（由 install.sh 生成，请勿手动修改）
BIN_PATH=${BIN_PATH}
PORT=${PORT}
DATA_DIR=${DATA_DIR}
EOF
chmod 0644 "${RECORD_FILE}"
ok "已写入安装记录 ${gl_bai}${RECORD_FILE}${reset}"

# 只提取版本号，过滤掉大 ASCII banner
"${BIN_PATH}" -version 2>/dev/null | grep -E "版本|v[0-9]+\.[0-9]+\.[0-9]+" || true

sep_line
section "启动服务"
if [ "${USE_SYSTEMD}" = "y" ]; then
  cat > "${SERVICE_FILE}" <<UNIT
[Unit]
Description=2Panel - 计划任务管理工具
After=network-online.target local-fs.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=${BIN_PATH} -port ${PORT} -data ${DATA_DIR}
WorkingDirectory=${DATA_DIR}
Environment=TZ=Asia/Shanghai
Restart=on-failure
RestartSec=3

[Install]
WantedBy=multi-user.target
UNIT

  systemctl daemon-reload
  systemctl enable "${APP_NAME}" >/dev/null 2>&1 || true
  systemctl restart "${APP_NAME}"
  sleep 2
  if systemctl is-active "${APP_NAME}" >/dev/null 2>&1; then
    ok "${gl_bai}${APP_NAME}${reset} 服务已启动。"
    systemctl status "${APP_NAME}" --no-pager || true
  else
    printf "  %s\n" "${gl_hong}[错误]${reset} 服务启动失败，请检查：${gl_bai}journalctl -u ${APP_NAME} -n 50${reset}" >&2
    exit 1
  fi
else
  if command -v pgrep >/dev/null 2>&1 && pgrep -x "${APP_NAME}" >/dev/null 2>&1; then
    printf "  %s\n" "${gl_huang}[警告]${reset} 检测到 ${APP_NAME} 进程可能已在运行"
  else
    nohup "${BIN_PATH}" -port "${PORT}" -data "${DATA_DIR}" >> "${DATA_DIR}/${APP_NAME}.log" 2>&1 &
    ok "${APP_NAME} 已在后台启动，pid: ${gl_bai}$!${reset}"
  fi
fi

# 取第一个IPv4
IP=$(hostname -I 2>/dev/null | awk '{print $1}')
[ -z "${IP}" ] && IP="<服务器IP>"

open_firewall_port "${PORT}"

if [ "${FW_OPENED}" = "y" ]; then
  FW_STATUS="${gl_lv}已开放 ${PORT}/tcp${reset}"
else
  FW_STATUS="${gl_huang}未检测到活跃防火墙，已跳过${reset}"
fi

sep_line
if [ "${USE_SYSTEMD}" = "y" ]; then
  printf "  %s\n" "${gl_lv}✔ ${APP_NAME} 安装成功！${reset}"
  printf "  %-14s %s\n" "${gl_lan}访问地址${reset}" "${gl_bai}http://${IP}:${PORT}${reset}"
  printf "  %-14s %s\n" "${gl_lan}数据目录${reset}" "${gl_bai}${DATA_DIR}${reset}"
  printf "  %-14s %s\n" "${gl_lan}二进制文件${reset}" "${gl_bai}${BIN_PATH}${reset}"
  printf "  %-14s %s\n" "${gl_lan}安装记录${reset}" "${gl_bai}${RECORD_FILE}${reset}"
  printf "  %-14s %s\n" "${gl_lan}防火墙状态${reset}" "$FW_STATUS"
  printf "  %-14s %s\n" "${gl_lan}运行模式${reset}" "${gl_bai}systemd 服务${reset}"
  sep_line
  printf "  %s\n" "${gl_bai}常用命令：${reset}"
  printf "    %-44s %s\n" "${gl_hui}systemctl status ${APP_NAME}${reset}" "${gl_lan}# 查看状态${reset}"
  printf "    %-44s %s\n" "${gl_hui}systemctl restart ${APP_NAME}${reset}" "${gl_lan}# 重启服务${reset}"
  printf "    %-44s %s\n" "${gl_hui}journalctl -u ${APP_NAME} -f${reset}" "${gl_lan}# 跟随日志${reset}"
  printf "    %-44s %s\n" "${gl_hui}journalctl -u ${APP_NAME} -n 50${reset}" "${gl_lan}# 最近日志${reset}"
else
  printf "  %s\n" "${gl_lv}✔ ${APP_NAME} 安装成功！${reset} ${gl_huang}（后台运行模式）${reset}"
  printf "  %-14s %s\n" "${gl_lan}访问地址${reset}" "${gl_bai}http://${IP}:${PORT}${reset}"
  printf "  %-14s %s\n" "${gl_lan}数据目录${reset}" "${gl_bai}${DATA_DIR}${reset}"
  printf "  %-14s %s\n" "${gl_lan}二进制文件${reset}" "${gl_bai}${BIN_PATH}${reset}"
  printf "  %-14s %s\n" "${gl_lan}防火墙状态${reset}" "$FW_STATUS"
  printf "  %-14s %s\n" "${gl_lan}运行模式${reset}" "${gl_huang}后台运行${reset}"
  printf "  %s\n" "  ${gl_huang}注意：${reset}后台运行模式在系统重启后不会自动恢复。"
  printf "  %s\n" "${gl_hui}    如需开机自启 / 崩溃自动重启 / journald 日志，请在安装 systemd 后重新运行本脚本。${reset}"
fi

sep_line