#!/usr/bin/env bash
# version-check.sh — api-pack 线上版本自查（只读）
#
# 回答一个问题："这台机现在跑的是哪一版，相对上游最新 Release 落后多少。"
# 判定依据是**二进制的 sha256**去对账 GitHub Release 资产的 digest，
# 而不是文件大小 / mtime —— 后两者能被手工替换、传输截断、多次覆盖搞脏，
# 而 size 相同的两个版本真实存在（见 --help 里的 caveat 说明）。
#
# 为什么不需要二进制内嵌版本号：api-pack 没有 version subcommand，
# 线上版本的真身就是"最新 v* tag 的那个 Release 资产"（见 DOCS/handoff.md）。
# 所以本脚本反查：本地二进制的 sha256 等于历史上哪个资产的 sha256。
#
# 纯只读：只读文件、只读 GitHub API。不写远端、不重启服务、不换二进制。
#
# 用法：
#   ./scripts/version-check.sh                     # 查本机（默认二进制 ~/api-pack-new）
#   ./scripts/version-check.sh -b /root/api-pack   # 查指定路径的二进制
#   ./scripts/version-check.sh --json              # 输出 JSON（给机器消费）
#   ./scripts/version-check.sh -q                  # 只输出 TSV 数据行，便于 diff/比对
#
# 远端用法（在机器上直接跑，见 DOCS/handoff.md "版本自查"）：
#   curl -sSL https://raw.githubusercontent.com/Hana-ame/api-pack/master/scripts/version-check.sh \
#     | bash -s -- -b /root/api-pack-new
#   （上面这条是只读的：不传 -b 时脚本只算 sha256 并查 GitHub，不碰任何写操作）

set -uo pipefail

REPO="${REPO:-Hana-ame/api-pack}"
ASSET="${ASSET:-myapp-linux-amd64}"
BIN="${BIN:-}"
FORMAT="tsv"
LOOKBACK="${LOOKBACK:-40}"
# 判定"落后"用的比较基线：release=比最新 Release；branch=比 origin/master HEAD
BASELINE="release"

usage() {
  cat <<'EOF'
usage: version-check.sh [-b BINARY] [--json|-q] [-n N] [--baseline release|branch]

  -b, --binary PATH   要检查的二进制路径（默认 ~/api-pack-new）
      --json          输出 JSON（默认 tsv）
  -q, --quiet         只输出数据行（无表头无注释），便于机器比对
  -n, --lookback N    往回查多少个历史 Release 来认版本（默认 40）
      --baseline R    release=比最新 Release（默认）；branch=比 origin/master
  -h, --help          本帮助

输出字段（TSV，制表符分隔，注释行以 # 开头）：
  host        本机 hostname（或 -H 指定）
  binary      二进制路径
  size_bytes  文件字节数
  sha256      二进制 sha256
  version     认出的 Release tag；认不出则 UNKNOWN
  upstream_tag     上游最新 Release tag
  upstream_sha      该 tag 指向的 commit
  upstream_digest   该 Release 资产的 sha256
  behind_count 比上游落后几个 Release（认不出版本时为 UNKNOWN）
  status      OK | BEHIND | UNKNOWN | AHEAD
  detail      人类可读的一句话说明

status 语义：
  OK        认出的版本 == 上游最新 Release 资产
  BEHIND    认出的版本落后于上游（behind_count > 0）
  AHEAD     认出的 tag 比上游还新（本地手工换过、或上游 Release 被删）
  UNKNOWN   二进制 sha256 不等于最近 N 个 Release 中任何一个
            （常见于手工编译上机、或 Release 已被重打）
EOF
}

HOST_OVERRIDE=""
JSON_OUT=0
QUIET=0

while [ $# -gt 0 ]; do
  case "$1" in
    -b|--binary) BIN="$2"; shift 2 ;;
    --json) JSON_OUT=1; shift ;;
    -q|--quiet) QUIET=1; shift ;;
    -n|--lookback) LOOKBACK="$2"; shift 2 ;;
    --baseline) BASELINE="$2"; shift 2 ;;
    -H|--host) HOST_OVERRIDE="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown arg: $1" >&2; usage >&2; exit 2 ;;
  esac
done

[ -n "$BIN" ] || BIN="$HOME/api-pack-new"

HOST="${HOST_OVERRIDE:-$(hostname 2>/dev/null || echo unknown)}"
SELF_SHA=""
SIZE=""

# --- 本地二进制指纹（只读） ---
if [ -f "$BIN" ]; then
  SIZE=$(wc -c < "$BIN" 2>/dev/null | tr -d ' ')
  if command -v sha256sum >/dev/null 2>&1; then
    SELF_SHA=$(sha256sum "$BIN" 2>/dev/null | cut -d' ' -f1)
  elif command -v shasum >/dev/null 2>&1; then
    SELF_SHA=$(shasum -a 256 "$BIN" 2>/dev/null | cut -d' ' -f1)
  else
    echo "ERROR: need sha256sum or shasum" >&2; exit 3
  fi
else
  echo "ERROR: binary not found: $BIN" >&2; exit 4
fi

# --- GitHub API 读数 ---
GH="curl -sS --max-time 20"
if command -v gh >/dev/null 2>&1; then
  # gh 已登录时能用更高限额，且不受匿名 API 限流影响
  GH="gh api"
fi

api_latest() { $GH "repos/$REPO/releases/latest"; }
api_tags()   { $GH "repos/$REPO/tags?per_page=100"; }

# --- tag 形态分类 -------------------------------------------------
# "版本对不上"至少有三类，必须分开报，否则没法处置：
#   semver    正常发布版本（v26.10.03）——参与正常比对与 behind_count
#   untagged  GitHub Actions 自动打的孤儿 tag（untagged-<40位hex>）。
#             它不是发布版本，永远不该被当成"线上该有的版本"；
#             出现即说明有 Actions 产物没被 tag 收敛掉。
#   other     其它形态（latest / flutter-lib-v1.0.0 这类非版本 tag）
# 注意：untagged-<sha> 出现在 **git refs** 里，通常不出现在 /releases，
# 所以只看 releases 会漏掉它——这正是要单独查 refs 的原因。
# 判据说明：
#  - 只认**严格三段数字** semver（v26.10.03）。
#  - "v0.0.0-<sha>" 这种形态是 GitHub Actions 的自动 tag（commit 被 tag 过
#    但没走发布流程），本仓实测存在 36 个。它们带 release，但**不是人类发布的版本**，
#    绝不能参与 behind_count——否则一次自动 tag 就会把"落后 N 个版本"算错。
#  - "untagged-<sha>" 是未收敛的 Actions 产物，**不该存在**；出现即残留。
classify_tag() {
  case "$1" in
    v[0-9]*.[0-9]*.[0-9]*)                  echo "semver" ;;
    untagged-*)                              echo "untagged" ;;
    v0.0.0-*)                               echo "orphan" ;;
    *)                                       echo "other" ;;
  esac
}

have_jq() { command -v jq >/dev/null 2>&1; }

if have_jq; then
  LATEST_JSON=$(api_latest 2>/dev/null)
  UPSTREAM_TAG=$(printf '%s' "$LATEST_JSON" | jq -r '.tag_name // empty')
  UPSTREAM_SHA=$(printf '%s' "$LATEST_JSON" | jq -r '.target_commitish // empty')
  UPSTREAM_DIGEST=$(printf '%s' "$LATEST_JSON" | jq -r --arg a "$ASSET" '.assets[]? | select(.name==$a) | (.digest // empty) | sub("^sha256:";"")')
  UPSTREAM_SIZE=$(printf '%s' "$LATEST_JSON" | jq -r --arg a "$ASSET" '.assets[]? | select(.name==$a) | (.size|tostring)')
  # tag -> sha（releases/latest 的 target_commitish 常常只是 "master"，
  # 拿不到真 sha，所以单独查 tags 列表）
  if [ -z "$UPSTREAM_SHA" ] || [ "$UPSTREAM_SHA" = "master" ] || [ "$UPSTREAM_SHA" = "main" ]; then
    UPSTREAM_SHA=$(api_tags 2>/dev/null | jq -r --arg t "$UPSTREAM_TAG" '.[]? | select(.name==$t) | .commit.sha')
  fi
  TAG_CLASS=$(classify_tag "$UPSTREAM_TAG")

  # 往回找：认版本
  VERSION="UNKNOWN"
  behind=""
  # releases 列表按时间倒序，第 N 个即落后 N-1 个版本
  RELS=$(api_tags 2>/dev/null >/dev/null; $GH "repos/$REPO/releases?per_page=$LOOKBACK" 2>/dev/null)
  n=0
  while IFS=$'\t' read -r tg dg; do
    n=$((n+1))
    # 只让 semver 参与"落后计数"：非版本 tag（latest / untagged-*）
    # 不是可部署版本，混进来会把 behind_count 算错。
    [ "$(classify_tag "$tg")" = "semver" ] || continue
    if [ "$dg" = "$SELF_SHA" ]; then VERSION="$tg"; behind=$((n-1)); break; fi
    if [ "$n" -ge "$LOOKBACK" ]; then break; fi
  done <<EOF
$(printf '%s' "$RELS" | jq -r --arg a "$ASSET" '.[]? | .tag_name as $t | (.assets[]? | select(.name==$a) | ($t + "\t" + ((.digest // "") | sub("^sha256:";""))))')
EOF

  # --- tag 形态盘点：草稿 / untagged 残留 ---
  # 扫全部 tag ref 的名字，不限 semver。untagged-* 只存在于 git refs，
  # /releases 看不到它，所以必须单独查。
  ALL_TAGS=$($GH "repos/$REPO/tags?per_page=100" 2>/dev/null | jq -r '.[].name' 2>/dev/null)
  # grep -c 在无匹配时输出 "0" 且退出码非 0，原写法把 "0" 和 echo 的 "0"
  # 拼成两行、连同换行一起进了变量——TSV 因此多出一列。
  # 统一用 awk 计数，永远只输出一个数字。
  UNTAGGED_TAGS=$(printf '%s\n' "$ALL_TAGS" | awk '/^untagged-/{c++} END{print c+0}')
  ORPHAN_TAGS=$(printf '%s\n' "$ALL_TAGS" | awk '/^v0\.0\.0-/{c++} END{print c+0}')
  # 其它非版本 tag：只报**数量**，不把名单塞进 TSV。
  # 本仓实测有 36 个 v0.0.0-*，逐个列出会让整行读不下去，
  # 而机器消费方只需要"有多少个异常形态"这个数。
  OTHER_COUNT=$(printf '%s\n' "$ALL_TAGS" | awk '!/^untagged-/ && !/^v0\.0\.0-/ && !/^v[0-9]+\.[0-9]+\.[0-9]+$/{c++} END{print c+0}')

  # 草稿 release：/releases 对草稿可见（draft=true），单列出来。
  DRAFT_TAGS=$($GH "repos/$REPO/releases?per_page=$LOOKBACK" 2>/dev/null | jq -r '[.[]? | select(.draft==true) | .tag_name] | join(",")' 2>/dev/null)
  [ -n "$DRAFT_TAGS" ] || DRAFT_TAGS="none"
else
  echo "ERROR: need jq (or install and re-run) for GitHub API parsing" >&2; exit 5
fi

# --- 判定 ---
if [ "$BASELINE" = "branch" ]; then
  # branch 基线：只报告认出的版本与 upstream tag，不给"落后几个 Release"
  STATUS="INFO"
  DETAIL="baseline=branch; 版本对账仍以 Release 资产 digest 为准"
  [ -n "$behind" ] || DETAIL="baseline=branch; 该 sha 未匹配最近 $LOOKBACK 个 Release"
else
  if [ "$VERSION" = "UNKNOWN" ]; then
    STATUS="UNKNOWN"
    DETAIL="sha256 未匹配最近 $LOOKBACK 个 Release 的 $ASSET（手工编译上机？或 Release 被重打）"
  elif [ -z "$UPSTREAM_DIGEST" ]; then
    STATUS="UNKNOWN"
    DETAIL="上游最新 Release 无 digest 字段（GitHub 侧未提供），无法对账"
  elif [ "$SELF_SHA" = "$UPSTREAM_DIGEST" ]; then
    STATUS="OK"
    DETAIL="与最新 Release $UPSTREAM_TAG 完全一致"
  else
    if [ "$behind" -gt 0 ]; then
      STATUS="BEHIND"
      DETAIL="落后上游 $behind 个 Release（线上 $VERSION，上游 $UPSTREAM_TAG）"
    else
      STATUS="AHEAD"
      DETAIL="认出的 $VERSION 不在上游最新 $UPSTREAM_TAG 之内"
    fi
  fi
fi

# --- 输出 ---
if [ "$JSON_OUT" = "1" ]; then
  jq -n --arg host "$HOST" --arg binary "$BIN" --arg size "$SIZE" \
        --arg sha256 "$SELF_SHA" --arg version "$VERSION" \
        --arg upstream_tag "$UPSTREAM_TAG" --arg upstream_sha "$UPSTREAM_SHA" \
        --arg upstream_digest "$UPSTREAM_DIGEST" --arg upstream_size "${UPSTREAM_SIZE:-}" \
        --arg behind "${behind:-}" --arg status "$STATUS" --arg detail "$DETAIL" \
        --arg tag_class "${TAG_CLASS:-unknown}" --arg draft "${DRAFT_TAGS:-none}" \
        --arg untagged "${UNTAGGED_TAGS:-0}" --arg orphan "${ORPHAN_TAGS:-0}" --arg other "${OTHER_COUNT:-0}" \
        '{host:$host,binary:$binary,size_bytes:($size|tonumber? // 0),sha256:$sha256,
          version:$version,upstream_tag:$upstream_tag,upstream_sha:$upstream_sha,
          upstream_digest:$upstream_digest,upstream_size:($upstream_size|tonumber? // 0),
          behind_count:($behind|tonumber? // null),status:$status,detail:$detail,
          tag_class:$tag_class,draft_releases:$draft,untagged_count:($untagged|tonumber? // 0),
          orphan_tag_count:($orphan|tonumber? // 0),other_tag_count:($other|tonumber? // 0)}'
else
  [ "$QUIET" = "1" ] || {
    echo "# api-pack version-check  host=$HOST  repo=$REPO  asset=$ASSET"
    echo "# fields: host binary size_bytes sha256 version upstream_tag upstream_sha upstream_digest behind_count status detail tag_class draft_releases untagged_count orphan_tags other_tags"
    echo "# tag_class(最新 release): semver=正常发布版本 | orphan=v0.0.0-<sha> 自动 tag | untagged=未收敛 Actions 产物 | other=其它"
    if [ "$UNTAGGED_TAGS" -gt 0 ] 2>/dev/null; then
      echo "# 注意：检出 $UNTAGGED_TAGS 个 untagged-* 残留 tag——它们不是发布版本，勿当作应部署版本"
    fi
    if [ "$DRAFT_TAGS" != "none" ]; then
      echo "# 注意：存在草稿 release: $DRAFT_TAGS"
    fi
    if [ "$ORPHAN_TAGS" -gt 0 ] 2>/dev/null; then
      echo "# 提示：$ORPHAN_TAGS 个 v0.0.0-<sha> 形态 tag（Actions 自动 tag，非人工发布版本，不参与落后计数）"
    fi
  }
  printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' \
    "$HOST" "$BIN" "${SIZE:-0}" "$SELF_SHA" "$VERSION" "$UPSTREAM_TAG" \
    "$UPSTREAM_SHA" "${UPSTREAM_DIGEST:-unknown}" "${behind:-UNKNOWN}" "$STATUS" "$DETAIL" \
    "${TAG_CLASS:-unknown}" "${DRAFT_TAGS:-none}" "${UNTAGGED_TAGS:-0}" "${ORPHAN_TAGS:-0}" "${OTHER_COUNT:-0}"
fi

case "$STATUS" in
  OK) exit 0 ;;
  BEHIND|AHEAD|UNKNOWN) exit 1 ;;
  *) exit 0 ;;
esac
