#!/usr/bin/env bash
#
# Build 2Panel release binaries for GitHub Releases.
# Produces ./dist/2panel_linux_amd64 and ./dist/2panel_linux_arm64
#
# Usage:
#   ./scripts/build-release.sh            # default version v0.1.0
#   VERSION=v1.0.0 ./scripts/build-release.sh
#
# Optional signing (OTA 方案 D, minisign):
#   MINISIGN_KEY=/path/to/2panel.minisign.key ./scripts/build-release.sh
#   -> produces dist/*.minisig; the public key must be pasted into
#      internal/upgrade/upgrade.go (MinisignPublicKey) before building the
#      binary that performs upgrades.
#
# Then create a release and upload the assets:
#   gh release create "$VERSION" dist/2panel_linux_amd64 dist/2panel_linux_arm64 \
#     --title "$VERSION" --notes "see README"

set -e

cd "$(dirname "$0")/.."

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

ok()     { printf "  %s %s\n" "${gl_lv}>>>${reset}" "$1"; }
warn()   { printf "  %s %s\n" "${gl_huang}[警告]${reset}" "$1" >&2; }
error()  { printf "  %s %s\n" "${gl_hong}[错误]${reset}" "$1" >&2; exit 1; }
sep_line() {
    printf '%s' "$gl_bufan"
    printf '—%.0s' {1..32}
    printf '%s\n' "$reset"
}

VERSION="${VERSION:-v0.1.1}"
BUILD="$(date -u +%Y%m%d%H%M%S)"
LDFLAGS="-s -w -X main.version=${VERSION} -X main.build=${BUILD} -X main.buildTime=${BUILD}"

clear
echo -e ""
echo -e "${gl_zi}>>> 构建 2Panel 二进制文件${gl_bai}"
echo -e "${gl_bufan}————————————————————————————————————————————————${gl_bai}"
    
SIGNING="n"
if [ -n "${MINISIGN_KEY}" ]; then
  if ! command -v minisign >/dev/null 2>&1; then
    error "MINISIGN_KEY 已设置但未找到 minisign 命令，请先安装（brew install minisign）"
  fi
  SIGNING="y"
  SIGN_TIME="$(date -u +%Y-%m-%dT%H:%M:%S%z)"
  ok "将对发布资产进行 minisign 签名（SIGN_TIME=${SIGN_TIME}）"
fi

mkdir -p dist

build() {
  local os="$1" arch="$2" name="$3"
  ok "building ${name} (${os}/${arch}) ..."
  CGO_ENABLED=0 GOOS="${os}" GOARCH="${arch}" \
    go build -trimpath -ldflags "${LDFLAGS}" -o "dist/${name}" .
  ok "checksum ${name}.sha256 ..."
  (cd dist && sha256sum "${name}" > "${name}.sha256")
  if [ "${SIGNING}" = "y" ]; then
    ok "signing ${name}.minisig ..."
    (cd dist && minisign -S -m "${name}" -x "${name}.minisig" -s "${MINISIGN_KEY}" -t "${SIGN_TIME}")
  fi
}

build linux amd64 2panel_linux_amd64
build linux arm64 2panel_linux_arm64

sep_line
printf "  %s\n" "${gl_lv}✔ 构建完成${reset}"
printf "%s\n" "${gl_hui}发布资产:${reset}"
(cd dist && ls -lh)
sep_line
if [ "${SIGNING}" = "y" ]; then
  printf "  %s\n" "${gl_bai}上传二进制 + .sha256 + .minisig 到 GitHub Release，例如：${reset}"
  echo "  gh release create ${VERSION} dist/2panel_linux_amd64 dist/2panel_linux_amd64.sha256 dist/2panel_linux_amd64.minisig dist/2panel_linux_arm64 dist/2panel_linux_arm64.sha256 dist/2panel_linux_arm64.minisig --title ${VERSION}"
else
  printf "  %s\n" "${gl_bai}上传二进制及其 .sha256 文件到 GitHub Release，例如：${reset}"
  echo "  gh release create ${VERSION} dist/2panel_linux_amd64 dist/2panel_linux_amd64.sha256 dist/2panel_linux_arm64 dist/2panel_linux_arm64.sha256 --title ${VERSION}"
fi
echo ""
printf "  %s\n" "${gl_huang}启用签名校验：${reset}"
echo "  minisign -G -p pub.minisign.pub -s 2panel.minisign.key"
echo "  将 pub.minisign.pub 内容粘贴到 internal/upgrade/upgrade.go 的 MinisignPublicKey 常量"
echo -e "${gl_bufan}————————————————————————————————————————————————${gl_bai}"
