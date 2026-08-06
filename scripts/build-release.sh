#!/usr/bin/env bash
#
# Build 2Panel release binaries for GitHub Releases.
# Produces ./dist/2panel_linux_amd64 and ./dist/2panel_linux_arm64
#
# Usage:
#   ./scripts/build-release.sh            # default version v0.1.0
#   VERSION=v1.0.0 ./scripts/build-release.sh
#
# Then create a release and upload the assets:
#   gh release create "$VERSION" dist/2panel_linux_amd64 dist/2panel_linux_arm64 \
#     --title "$VERSION" --notes "see README"

set -e

cd "$(dirname "$0")/.."

VERSION="${VERSION:-v0.1.1}"
BUILD="$(date -u +%Y%m%d%H%M%S)"
LDFLAGS="-s -w -X main.version=${VERSION} -X main.build=${BUILD}"

mkdir -p dist

build() {
  local os="$1" arch="$2" name="$3"
  echo ">>> building ${name} (${os}/${arch}) ..."
  CGO_ENABLED=0 GOOS="${os}" GOARCH="${arch}" \
    go build -trimpath -ldflags "${LDFLAGS}" -o "dist/${name}" .
  echo ">>> checksum ${name}.sha256 ..."
  (cd dist && sha256sum "${name}" > "${name}.sha256")
}

build linux amd64 2panel_linux_amd64
build linux arm64 2panel_linux_arm64

echo ""
echo ">>> done. Release assets:"
ls -lh dist/
echo ""
echo "Upload them to a GitHub Release (binaries AND their .sha256 files), e.g.:"
echo "  gh release create ${VERSION} dist/2panel_linux_amd64 dist/2panel_linux_amd64.sha256 dist/2panel_linux_arm64 dist/2panel_linux_arm64.sha256 --title ${VERSION}"
