#!/bin/bash
#
# 2panel - 发布脚本（触发 GitHub Actions 自动构建）
# 不在本地编译任何产物：仅更新版本号、推送代码并打 v 开头 tag。
# 推送 tag 后由 GitHub Actions 自动完成发布（单条 workflow run）：
#   release.yaml -> amd64/arm64 二进制 + sha256 创建 GitHub Release（版本跟随 main.go）
#                   + multi-arch Docker 镜像（latest + 版本标签）
#
# Usage:
#   TAG(必填) 形如 v1.0.0; --yes 免交互; -m "备注" 可选发版说明
#     bash scripts/build-and-push.sh v1.0.0 --yes -m "本次新增 xxx"
set -euo pipefail

info() { echo -e "\033[32m>>> $*\033[0m"; }
warn() { echo -e "\033[33m!!! $*\033[0m"; }
error() { echo -e "\033[31mERROR: $*\033[0m"; exit 1; }

YES_MODE=0
TAG=""
MSG=""
while [[ $# -gt 0 ]]; do
    case "$1" in
        --yes) YES_MODE=1; shift ;;
        -m|--message)
            shift
            [ -n "${1:-}" ] || error "缺少 -m/--message 的备注内容"
            MSG="$1"
            shift
            ;;
        *) TAG="$1"; shift ;;
    esac
done

[ -z "${TAG}" ] && error "缺少 TAG 参数，用法: bash scripts/build-and-push.sh v1.0.0 --yes -m \"备注\""
TARGET_VER="${TAG#v}"
VER_RE='^v[0-9]+\.[0-9]+\.[0-9]+$'
[[ "${TAG}" =~ ${VER_RE} ]] || error "TAG 必须形如 vX.Y.Z（当前: ${TAG}）"

cd "$(dirname "$0")/.."

if [ "${YES_MODE}" = "0" ]; then
    read -r -p "即将发布 ${TAG}，将 bump main.go 版本号并推送 main + tag，确认? [y/N] " ans
    [[ "${ans}" =~ ^[Yy]$ ]] || { echo "已取消"; exit 1; }
fi

[[ "$(git status --porcelain)" =~ .[MADR] ]] && {
    warn "工作区存在未提交改动:"
    git status --short
    error "请先提交或 stash 后再发布"
}

# ===================== 同步远端 =====================
info "fetch 远端状态"
git fetch origin
LOCAL_HASH=$(git rev-parse @)
REMOTE_HASH=$(git rev-parse @{u} 2>/dev/null || echo "")
if [ -n "${REMOTE_HASH}" ] && [ "${LOCAL_HASH}" != "${REMOTE_HASH}" ]; then
    error "本地 main 与远端不一致，请先 git pull"
fi

# ===================== bump main.go 版本号 =====================
info "更新 main.go 版本号 -> ${TAG}"
python3 - "${TARGET_VER}" <<'PY'
import re, sys
want = sys.argv[1]
p = 'main.go'
s = open(p, encoding='utf-8').read()
new, n = re.subn(r'^(\t{0,2}version\s*=\s*)"v[0-9.]+"',
                 lambda m: m.group(1) + '"v' + want + '"', s, count=1, flags=re.M)
if n == 0:
    print('ERROR: main.go 中未找到 version 定义'); sys.exit(1)
open(p, 'w', encoding='utf-8').write(new)
print('main.go version -> v' + want)
PY

git diff --stat

# ===================== 写发版备注 =====================
info "写入发版备注 RELEASE_NOTES.md"
{
  if [ -n "${MSG}" ]; then
    printf '%s\n' "${MSG}"
  fi
} > RELEASE_NOTES.md

# ===================== Git 提交 & Tag =====================
info "提交版本变更"
git add main.go RELEASE_NOTES.md
git commit -q -m "chore: bump version to ${TAG}" || warn "无变更可提交？"
info "推送 main"
git push origin main
if git rev-parse -q --verify "refs/tags/${TAG}" >/dev/null; then
    if [ "${YES_MODE}" = "0" ]; then
        read -r -p "远端已存在 tag ${TAG}，删除并重建? [y/N] " ans
        [[ "${ans}" =~ ^[Yy]$ ]] || error "已取消（tag ${TAG} 已存在）"
    fi
    git tag -f -a "${TAG}" -m "${TAG}"
    git push origin ":refs/tags/${TAG}" --force
    git push origin "refs/tags/${TAG}" --force
else
    git tag -a "${TAG}" -m "${TAG}"
    git push origin "refs/tags/${TAG}"
fi

info "完成！GitHub Actions 将自动构建并发布："
info "  gh run list --workflow=release.yaml --limit 1"
info "  gh release view ${TAG}"
info "  docker pull mobufan/2panel:${TAG}"