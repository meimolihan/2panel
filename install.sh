#!/usr/bin/env bash
#
# 2Panel - Scheduled Task Manager
# One-line remote installer.
#
# Usage:
#   bash -c "$(curl -sSL https://raw.githubusercontent.com/meimolihan/2Panel/main/install.sh)"
#
# Requires a GitHub Release containing the binaries built by scripts/build-release.sh.

set -e

# ================== customize me ==================
GITHUB_OWNER="meimolihan"             # your GitHub username
GITHUB_REPO="2Panel"                  # GitHub repository name
DEFAULT_PORT=8080
DEFAULT_DATA_DIR="/var/lib/2panel"
# ==================================================

API_BASE="https://api.github.com/repos/${GITHUB_OWNER}/${GITHUB_REPO}"

print_banner() {
  cat <<'EOF'
 ____  ____                  _
|___ \|  _ \ __ _ _ __   ___| |
  __) | |_) / _` | '_ \ / _ \ |
 / __/|  __/ (_| | | | |  __/ |
|_____|_|   \__,_|_| |_|\___|_|

2Panel - 计划任务管理工具
EOF
}

error() { echo "[错误] $1" >&2; exit 1; }

# ---- firewall: automatically open the listen port ----
# 依次尝试 firewalld → ufw → iptables，仅放行已实际启用的防火墙。
# 结果写入 FW_OPENED（y/n），供安装横幅展示真实状态。
FW_OPENED="n"
open_firewall_port() {
  local PORT="$1"

  # 1. firewalld
  if command -v firewall-cmd >/dev/null 2>&1 && firewall-cmd --state >/dev/null 2>&1; then
    if ! firewall-cmd --query-port="${PORT}/tcp" >/dev/null 2>&1; then
      firewall-cmd --permanent --add-port="${PORT}/tcp" >/dev/null 2>&1 || true
      firewall-cmd --reload >/dev/null 2>&1 || true
    fi
    echo ">>> 已通过 firewalld 开放端口 ${PORT}/tcp"
    FW_OPENED="y"
    return 0
  fi

  # 2. ufw
  if command -v ufw >/dev/null 2>&1 && ufw status 2>/dev/null | grep -q "Status: active"; then
    if ! ufw status 2>/dev/null | grep -q "${PORT}/tcp"; then
      ufw allow "${PORT}/tcp" >/dev/null 2>&1 || true
    fi
    echo ">>> 已通过 ufw 开放端口 ${PORT}/tcp"
    FW_OPENED="y"
    return 0
  fi

  # 3. iptables
  if command -v iptables >/dev/null 2>&1; then
    if iptables -C INPUT -p tcp --dport "${PORT}" -j ACCEPT >/dev/null 2>&1; then
      echo ">>> 端口 ${PORT}/tcp 已在 iptables 中放行"
      FW_OPENED="y"
      return 0
    fi
    # 仅当 INPUT 链存在实际过滤策略时才添加规则，避免画蛇添足
    if iptables -L INPUT -n 2>/dev/null | grep -qE 'policy (DROP|REJECT)|REJECT|DROP'; then
      if iptables -I INPUT -p tcp --dport "${PORT}" -j ACCEPT >/dev/null 2>&1; then
        echo ">>> 已通过 iptables 开放端口 ${PORT}/tcp"
        FW_OPENED="y"
        return 0
      fi
    fi
  fi

  echo "[提示] 未检测到活跃的防火墙（firewalld/ufw/iptables），跳过端口开放。"
}

# ---- preflight ----
[ "$(id -u)" != "0" ] && error "请以 root 身份运行（例如 sudo bash -c \"$(curl -sSL ...)\"）"

command -v curl >/dev/null 2>&1 || error "请先安装 curl（apt install curl / yum install curl）"

ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64) ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *) error "不支持的架构: $ARCH（仅支持 amd64 / arm64）" ;;
esac

print_banner
echo "============================================================"
echo " 正在安装 2Panel"
echo "   系统 : $(uname -s) $(uname -m)"
echo "   架构 : $ARCH"
echo "============================================================"

# ---- prompt: listen port ----
while :; do
  read -r -p "请输入监听端口 [默认: ${DEFAULT_PORT}]: " PORT
  PORT="${PORT:-$DEFAULT_PORT}"
  case "$PORT" in
    ''|*[!0-9]*) echo "  端口无效，请重新输入。" ;;
    *)
      if [ "$PORT" -ge 1 ] && [ "$PORT" -le 65535 ]; then
        break
      fi
      echo "  端口超出范围（1-65535），请重新输入。"
      ;;
  esac
done

# ---- prompt: data directory ----
read -r -p "请输入数据目录 [默认: ${DEFAULT_DATA_DIR}]: " DATA_DIR
DATA_DIR="${DATA_DIR:-$DEFAULT_DATA_DIR}"

# ---- systemd: auto-configure (preferred) ----
# systemd 提供开机自启、崩溃自动重启与 journald 统一日志；
# 仅当系统没有 systemd（如容器环境）时才回退为后台运行模式。
if command -v systemctl >/dev/null 2>&1; then
  USE_SYSTEMD="y"
else
  USE_SYSTEMD="n"
  echo ""
  echo "[警告] 未检测到 systemd（容器或受限环境）。"
  echo "       已回退为后台运行模式，重启或崩溃后服务不会自动恢复。"
fi

BIN_DIR="/usr/local/bin"
BIN_PATH="${BIN_DIR}/2panel"
BIN_URL="https://github.com/${GITHUB_OWNER}/${GITHUB_REPO}/releases/latest/download/2panel_linux_${ARCH}"

echo ""
echo ">>> 正在下载 2panel (${ARCH}) ..."
curl -fSL --retry 3 -o "${BIN_PATH}.download" "${BIN_URL}" || error "下载失败，请检查 GITHUB_OWNER 与 GitHub Release 附件"
chmod +x "${BIN_PATH}.download"
mv "${BIN_PATH}.download" "${BIN_PATH}"

echo ">>> 正在创建数据目录 ${DATA_DIR} ..."
mkdir -p "${DATA_DIR}"
chmod 700 "${DATA_DIR}"

"${BIN_PATH}" -version

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
    echo ">>> 2panel 服务已启动。"
    systemctl status 2panel --no-pager || true
  else
    echo "[错误] 服务启动失败，请检查：journalctl -u 2panel -n 50" >&2
    exit 1
  fi
else
  if command -v pgrep >/dev/null 2>&1 && pgrep -f "${BIN_PATH} -port ${PORT}" >/dev/null 2>&1; then
    echo "[警告] 2panel 可能已在端口 ${PORT} 上运行"
  else
    nohup "${BIN_PATH}" -port "${PORT}" -data "${DATA_DIR}" >> "${DATA_DIR}/2panel.log" 2>&1 &
    echo ">>> 2panel 已在后台启动，pid: $!"
  fi
fi

IP=$(hostname -I 2>/dev/null | awk '{print $1}')
[ -z "$IP" ] && IP="<服务器IP>"

open_firewall_port "$PORT"

if [ "$FW_OPENED" = "y" ]; then
  FW_LINE="    防火墙   : 已开放 ${PORT}/tcp"
else
  FW_LINE="    防火墙   : 未检测到活跃防火墙，已跳过"
fi

if [ "$USE_SYSTEMD" = "y" ]; then
  cat <<EOF

============================================================
  2Panel 安装成功！

    Web UI   : http://${IP}:${PORT}
    数据目录 : ${DATA_DIR}
    二进制   : ${BIN_PATH}
    版本     : ${BIN_PATH} -version
    ${FW_LINE}

  已注册为 systemd 服务 "2panel":
    systemctl status 2panel      # 查看服务状态
    systemctl restart 2panel     # 重启服务
    journalctl -u 2panel -f      # 查看实时日志
    systemctl disable 2panel     # 关闭开机自启
============================================================
EOF
else
  cat <<EOF

============================================================
  2Panel 安装成功！（后台运行模式）

    Web UI   : http://${IP}:${PORT}
    数据目录 : ${DATA_DIR}
    二进制   : ${BIN_PATH}
    版本     : ${BIN_PATH} -version
    ${FW_LINE}

  注意：后台运行模式在系统重启后不会自动恢复。
        如需开机自启 / 崩溃自动重启 / journald 日志，
        请在安装 systemd 后重新运行本脚本。
============================================================
EOF
fi
