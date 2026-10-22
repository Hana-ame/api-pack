#!/usr/bin/env bash
# build.sh — 给 api-pack 打 tag 并 force-push，触发 CI 构建 release。
# 部署一律走 tag→CI→release 资产（myapp-linux-amd64），不再本地 build+scp。
#
# 用法：
#   ./build.sh              # 用今天的版本 v<yy.mm.dd>，如 v26.10.23
#   ./build.sh v26.10.23    # 指定 tag
#
# 行为：
#   1. 把当前 commit 推到 origin/master（tag 应落在已推送的 commit 上）
#   2. force-move 本地 tag 到 HEAD
#   3. 删掉同名旧 release（best-effort；CI 会用新 tag 重建资产）
#   4. git push --force origin <tag>  → 触发 .github/workflows/release.yml
set -euo pipefail

TAG="${1:-v$(date +%y.%m.%d)}"
cd "$(git rev-parse --show-toplevel)"

echo "==> $TAG @ $(git rev-parse --short HEAD)"

# 1. 当前 commit 推到 origin/master
git push origin HEAD:master

# 2. force-move 本地 tag 到 HEAD
git tag -f "$TAG"

# 3. 删掉同名旧 release（CI 会用新 tag 重新创建；release 不存在则忽略）
gh release delete "$TAG" --yes 2>/dev/null || true

# 4. force-push tag → 触发 release.yml 构建 myapp-linux-amd64
git push --force origin "$TAG"

echo "✅ $TAG -> $(git rev-parse HEAD)"
echo "CI:   https://github.com/Hana-ame/api-pack/actions"
echo "asset: myapp-linux-amd64（cloudcone 部署时改名 api-pack-new）"
