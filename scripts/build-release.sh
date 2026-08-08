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

VERSION="${VERSION:-v0.1.1}"
BUILD="$(date -u +%Y%m%d%H%M%S)"
LDFLAGS="-s -w -X main.version=${VERSION} -X main.build=${BUILD}"

# ---- minisign signing (optional, 方案 D) ----
SIGNING="n"
if [ -n "${MINISIGN_KEY}" ]; then
  if ! command -v minisign >/dev/null 2>&1; then
    echo "!!! MINISIGN_KEY 已设置但未找到 minisign 命令，请先安装（brew install minisign）" >&2
    exit 1
  fi
  SIGNING="y"
  SIGN_TIME="$(date -u +%Y-%m-%dT%H:%M:%S%z)"
  echo ">>> 将对发布资产进行 minisign 签名（SIGN_TIME=${SIGN_TIME}）"
fi

mkdir -p dist

build() {
  local os="$1" arch="$2" name="$3"
  echo ">>> building ${name} (${os}/${arch}) ..."
  CGO_ENABLED=0 GOOS="${os}" GOARCH="${arch}" \
    go build -trimpath -ldflags "${LDFLAGS}" -o "dist/${name}" .
  echo ">>> checksum ${name}.sha256 ..."
  (cd dist && sha256sum "${name}" > "${name}.sha256")
  if [ "${SIGNING}" = "y" ]; then
    echo ">>> signing ${name}.minisig ..."
    (cd dist && minisign -S -m "${name}" -x "${name}.minisig" -s "${MINISIGN_KEY}" -t "${SIGN_TIME}")
  fi
}

build linux amd64 2panel_linux_amd64
build linux arm64 2panel_linux_arm64

echo ""
echo ">>> done. Release assets:"
ls -lh dist/
echo ""
if [ "${SIGNING}" = "y" ]; then
  echo "上传二进制 + .sha256 + .minisig（校验与验签均依赖这些文件）:"
  echo "  gh release create ${VERSION} dist/2panel_linux_amd64 dist/2panel_linux_amd64.sha256 dist/2panel_linux_amd64.minisig dist/2panel_linux_arm64 dist/2panel_linux_arm64.sha256 dist/2panel_linux_arm64.minisig --title ${VERSION}"
else
  echo "Upload them to a GitHub Release (binaries AND their .sha256 files), e.g.:"
  echo "  gh release create ${VERSION} dist/2panel_linux_amd64 dist/2panel_linux_amd64.sha256 dist/2panel_linux_arm64 dist/2panel_linux_arm64.sha256 --title ${VERSION}"
fi
echo ""
echo ">>> 启用签名校验："
echo "    minisign -G -p pub.minisign.pub -s 2panel.minisign.key"
echo "    将 pub.minisign.pub 内容粘贴到 internal/upgrade/upgrade.go 的 MinisignPublicKey 常量"
