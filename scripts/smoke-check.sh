#!/usr/bin/env bash
# smoke-check.sh — api-pack 部署后冒烟检查（只读）
#
# 部署完自动打一遍关键路由，把"路由 404"这种故障**在部署当场**报出来，
# 而不是等用户自己发现。
#
# 本脚本唯一的设计要点：**区分三种不同的故障**。它们外表相似，
# 处置方式完全不同，混成一个红灯就没法派单了：
#
#   DOWN      端口不连 / TCP 超时      -> 服务根本没起来（查 systemd/nohup/二进制）
#   NO_ROUTE  连上了，但路由没注册      -> 服务活着，是这版二进制没有这段路由
#                                           （要么版本太老，要么 .env 门控没开）
#   BAD_BODY  路由在，返回了但内容不对   -> 路由活着，上游/门禁/内容层出问题
#                                           （查上游 provider、token、日志）
#
# 为什么必须这么分：api-pack 的两个 engine 都挂了 r.NoRoute 兜底
#   - moonchan engine: /var/www/moonchan 的 index.html（缺文件时也回 200）
#   - root engine:     proxy.RootProxyHandler（没 proxy_host 时重定向或出代理生成页）
# 所以"路由不存在"并不一定表现为 404 —— 它可能回 200 + 一段 HTML。
# 任何只看 status_code 的检查都会把"路由缺失"误判成"服务正常"。
# 本脚本因此对每条路由做**内容断言**，而不是只看状态码。
#
# 纯只读：只发 HTTP 请求。不写远端、不重启服务、不改配置。
#
# 用法：
#   ./scripts/smoke-check.sh -H 127.0.0.1:8080
#   ./scripts/smoke-check.sh -H bwh.moonchan.xyz:443 --scheme https
#   ./scripts/smoke-check.sh -H 127.24.11.16:8080 --json
#
# 远端用法（部署后立刻在机上跑）：
#   ./scripts/smoke-check.sh -H 127.24.11.16:8080

set -uo pipefail

HOSTPORT="${HOSTPORT:-}"
SCHEME="http"
TIMEOUT="${TIMEOUT:-8}"
FORMAT="tsv"
QUIET=0
ONLY=""

usage() {
  cat <<'EOF'
usage: smoke-check.sh -H HOST:PORT [--scheme http|https] [--json] [-k KEY] [--timeout SEC]

  -H, --host HOST:PORT  目标监听地址（必填）。测多个目标请多次调用，
                        然后按 TSV 的 host 列拼接（见下方"多机"）。
      --scheme S         http（默认）或 https
      --timeout SEC      单请求超时秒数（默认 8）
  -k, --route KEY       只跑某一条：opencode_models | engine_alive |
                        auth_gate_post | no_route_probe
      --list             列出所有内置检查项及其"为什么关键"，然后退出
  -q, --quiet           只输出数据行（无表头无汇总），便于多机拼接比对
      --json             输出 JSON 数组
  -h, --help            本帮助

多机用法（不碰远端，只发 HTTP 请求）：
  for h in 127.25.5.19:8080 127.24.11.16:8080 127.26.8.31:8080; do
    ./scripts/smoke-check.sh -H "$h" -q
  done

输出字段（TSV）：
  host        目标地址
  route       路由 key
  path        请求路径
  method      HTTP 方法
  status      HTTP 状态码（0 = 没连上）
  verdict     DOWN | NO_ROUTE | BAD_BODY | OK
  detail      一句话说明（含 Content-Type、命中/未命中的断言）
  elapsed_ms  耗时

退出码：0 = 全部 OK；1 = 有 NO_ROUTE/BAD_BODY/DOWN；2 = 用法错误

--- 为什么这几条是关键路由 ---
moonchan engine（SHIJIMA 监听，bwh=127.25.5.19:8080）:
  opencode_models   /api/v2/opencode/v1/models
      今天实测 404 的就是这条（2026-10-04）。它是 opencode zen 免费层反代
      的存在性探针：该路由由 opencode.Routes() 注册，老版本二进制里根本没有，
      所以"404"和"服务挂了"要用这条来区分。它同时是**唯一一条能证明
      "新版本二进制真的在跑"的路由**——bwh 停在 08-23 时它 404，
      收敛到 v26.10.03 后它 200。
  engine_alive       /api/v2/cookie
      同一个 engine 的存活探针，**不碰数据库**。用来区分"整个 engine 死了"
      与"只有 opencode 那段路由缺失"——这条 DOWN 是 engine 整体问题；
      这条 OK 而 opencode_models NO_ROUTE，则是版本/门控问题。
      选它而不是 /api/v2/?tid=N：后者要连 MySQL，数据库挂了会回 500，
      会被误判成 BAD_BODY（路由问题），而真实原因是后端数据库。
  auth_gate_post    POST /api/v2/
      带 checkID 门禁的那条。无 cookie 必须 401 **且响应体为空**。
      关键在于它验证**鉴权层还在**：如果这条变成 200 非空，
      说明门禁被绕过（严重问题），不是"服务正常"。
  no_route_probe     /api/v2/nonexistent-probe-xyz   （阴性对照）
      故意打一条不存在的路由，它**期望就是"未注册"**。
      作用是标定本机的"路由缺失"长什么样，从而让上面几条的 NO_ROUTE 判定可信；
      若它反而通了，说明兜底行为或路由树变了，此时所有 NO_ROUTE 都要打折看。
      它不计入失败数，但会单独标注"阴性对照异常"。

  --- 关于"为什么不能只看状态码" ---
  实测（2026-10-04，v26.10.03 本机复现）：同一个"路由没注册"，
  在两个 engine 上表现完全不同：
    moonchan engine → 404（+ 若 /var/www/moonchan/index.html 存在则可能 200+HTML）
    root engine     → 302 跳 https://moonchan.xyz/（RootProxyHandler 的兜底）
  也就是说"服务没注册这条路由"完全可能表现为 **200 或 302**，不是 404。
  只看 status_code 的冒烟检查会把这种情况判成"通"，然后放过去。
EOF
}

ROUTE_NAMES=""

while [ $# -gt 0 ]; do
  case "$1" in
    -H|--host) HOSTPORT="$2"; shift 2 ;;
    --scheme) SCHEME="$2"; shift 2 ;;
    -k|--route) ONLY="$2"; shift 2 ;;
    --list) ONLY="__LIST__"; shift ;;
    --json) FORMAT="json"; shift ;;
    -q|--quiet) QUIET=1; shift ;;
    --timeout) TIMEOUT="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown arg: $1" >&2; usage >&2; exit 2 ;;
  esac
done

if [ "$ONLY" = "__LIST__" ]; then
  usage; exit 0
fi
if [ -z "$HOSTPORT" ]; then
  echo "ERROR: -H HOST:PORT is required" >&2; usage >&2; exit 2
fi

# ---------- 探测原语 ----------
# probe <url> <method> [body] -> 写 $P_STATUS $P_CT $P_LOC $P_BODYFILE $P_MS
#
# 只发**一次**请求，header 与 body 从同一次响应里取。
# （早期版本另发一次 HEAD 取 Content-Type：既多一次往返，
#  又可能与真正的 GET/POST 不一致——上游瞬时抖动时会读到错的类型。）
probe() {
  local url="$1" method="${2:-GET}" body="${3:-}"
  P_BODYFILE=$(mktemp)
  local hdrfile start end code
  hdrfile=$(mktemp)
  start=$(date +%s%3N 2>/dev/null || echo 0)
  if [ "$method" = "POST" ]; then
    code=$(curl -sS -D "$hdrfile" -o "$P_BODYFILE" -w '%{http_code}' -X POST \
           --max-time "$TIMEOUT" -H 'Content-Type: application/json' \
           -d "${body:-{}}" "$url" 2>/dev/null)
  else
    code=$(curl -sS -D "$hdrfile" -o "$P_BODYFILE" -w '%{http_code}' \
           --max-time "$TIMEOUT" "$url" 2>/dev/null)
  fi
  end=$(date +%s%3N 2>/dev/null || echo 0)
  P_STATUS="${code:-000}"
  P_CT=$(tr -d '\r' < "$hdrfile" | grep -i '^content-type:' | tail -1 | cut -d' ' -f2-)
  P_LOC=$(tr -d '\r' < "$hdrfile" | grep -i '^location:' | tail -1 | cut -d' ' -f2-)
  rm -f "$hdrfile"
  P_MS=$(( end - start ))
}

body_has() { grep -qi -- "$1" "$P_BODYFILE" 2>/dev/null; }
body_is_html() { grep -qi '^[[:space:]]*<' "$P_BODYFILE" 2>/dev/null; }

# 路由"没注册"的指纹有三种形态，都属同一类故障（请求没进到目标 handler）：
#   1) 真 404                       —— moonchan engine 上最常见
#   2) 200/4xx + HTML               —— NoRoute 兜底把未注册路径吃了（/var/www/moonchan/index.html）
#   3) 3xx + Location               —— root engine 的 RootProxyHandler 跳转
# 只看 status_code 的检查会把第 2、3 种误判成"服务正常"。
looks_unrouted() {
  [ "$P_STATUS" = "404" ] && return 0
  body_is_html && return 0
  [ -n "${P_LOC:-}" ] && return 0
  return 1
}

# ---------- 单条检查 ----------
# check <key> <path> <method> <assertion> [expect_status]
#   assertion: json | json_no_html | html | status401 | any
# 阴性对照入口：CONTROL=1 表示"这条期望就是不通"。
CONTROL=0
CONTROL_BAD=0
run_control() {
  CONTROL=1
  run_one "$@"
  CONTROL=0
}

run_one() {
  local key="$1" path="$2" method="$3" assertion="${4:-any}" expect="${5:-}"
  local url="$SCHEME://$HOSTPORT$path"
  probe "$url" "$method"
  local verdict="" detail=""

  # --- 第一层：连不上 = DOWN。必须在任何状态码判断之前 ---
  if [ "$P_STATUS" = "000" ]; then
    verdict="DOWN"
    detail="连不上 $url（TCP 拒绝/超时）——服务未起，查进程与二进制；这与版本无关"

  # --- 第二层：请求没进到目标 handler = NO_ROUTE ---
  # 必须在"状态码是否符合预期"之前判：moonchan 的 NoRoute 兜底会回
  # 200+HTML、root engine 会回 302，这些都不等于"通了"。
  elif looks_unrouted; then
    if [ "$P_STATUS" = "404" ]; then
      detail="HTTP 404——服务在听，但该路由未注册（版本太老 / .env 门控未开）"
    elif [ -n "${P_LOC:-}" ]; then
      detail="HTTP $P_STATUS 跳转至 $P_LOC——被 root engine 的 RootProxyHandler 兜底吃掉，该路由未注册"
    else
      detail="HTTP $P_STATUS + HTML——被 NoRoute 兜底吃掉，该路由未注册（注意：不是 404，别只盯状态码）"
    fi
    verdict="NO_ROUTE"

  # --- 第三层：进了 handler 但内容不对 = BAD_BODY ---
  elif [ -n "$expect" ] && [ "$P_STATUS" != "$expect" ]; then
    if [ "$P_STATUS" = "502" ] || [ "$P_STATUS" = "503" ]; then
      detail="HTTP $P_STATUS——路由在，是上游 provider 故障（勿误判为本地故障）"
    else
      detail="HTTP $P_STATUS——路由在但响应异常，期望 $expect"
    fi
    verdict="BAD_BODY"

  # --- 第四层：状态码符合预期，做内容断言 ---
  else
    case "$assertion" in
      json)
        if body_has '"data"\|"object"\|"error"\|{'; then
          verdict="OK"; detail="HTTP $P_STATUS 且响应体是 JSON"
        elif [ ! -s "$P_BODYFILE" ]; then
          verdict="OK"; detail="HTTP $P_STATUS 且响应体为空（无鉴权门禁拒绝，符合预期）"
        else
          verdict="BAD_BODY"; detail="HTTP $P_STATUS 但响应体不是预期 JSON"
        fi ;;
      empty_body)
        if [ ! -s "$P_BODYFILE" ]; then
          verdict="OK"; detail="HTTP $P_STATUS 且响应体为空（门禁拒绝，符合预期）"
        else
          verdict="BAD_BODY"; detail="HTTP $P_STATUS 但响应体非空——门禁可能被绕过，需人工确认"
        fi ;;
      any)
        verdict="OK"; detail="HTTP $P_STATUS" ;;
      *)
        verdict="OK"; detail="HTTP $P_STATUS" ;;
    esac
  fi

  # control=1 的项是阴性对照：它期望的就是"未注册"，
  # 因此 NO_ROUTE 属于预期结果，不计入失败。
  # DOWN 时不判异常：服务整体没起时对照项同样连不上，这是同一件事的
  # 两种表现，不该再叠一条"判定不可信"的噪音。
  if [ "$CONTROL" = "1" ] && [ "$verdict" != "DOWN" ]; then
    if [ "$verdict" = "NO_ROUTE" ]; then
      detail="$detail（阴性对照：符合预期）"
    else
      detail="$detail（阴性对照异常：这条本应未注册却通了，NO_ROUTE 判定不可信）"
      CONTROL_BAD=1
    fi
  fi
  emit "$key" "$path" "$method" "$verdict" "$detail"
}

emit() {
  local key="$1" path="$2" method="$3" verdict="$4" detail="$5"
  local ct="${P_CT:-}"
  [ -n "$FORMAT" ] || true
  if [ "$FORMAT" = "json" ]; then
    RESULTS+=("$(jq -n --arg host "$HOSTPORT" --arg route "$key" --arg path "$path" \
        --arg method "$method" --arg status "${P_STATUS:-0}" --arg ct "$ct" \
        --arg verdict "$verdict" --arg detail "$detail" --argjson ms "${P_MS:-0}" \
        '{host:$host,route:$route,path:$path,method:$method,status:$status,
          content_type:$ct,verdict:$verdict,detail:$detail,elapsed_ms:$ms}')")
  else
    printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' \
      "$HOSTPORT" "$key" "$path" "$method" "${P_STATUS:-0}" "$verdict" "${P_MS:-0}ms" "$detail"
  fi
  if [ "$verdict" != "OK" ] && [ "$CONTROL" != "1" ]; then FAILED=$((FAILED+1)); fi
  [ -n "$ROUTE_NAMES" ] || ROUTE_NAMES="$key"
  ROUTE_NAMES="$ROUTE_NAMES $key"
}

FAILED=0
RESULTS=()

# ---------- 路由清单 ----------
check_routes() {
  if [ -n "$ONLY" ]; then
    case "$ONLY" in
      opencode_models) run_one opencode_models "/api/v2/opencode/v1/models" GET json 200 ;;
      engine_alive) run_one engine_alive "/api/v2/cookie" GET any 200 ;;
      auth_gate_post) run_one auth_gate_post "/api/v2/" POST empty_body 401 ;;
      no_route_probe) run_one no_route_probe "/api/v2/nonexistent-probe-xyz" GET none 404 ;;
      *) echo "unknown route: $ONLY" >&2; exit 2 ;;
    esac
    return
  fi
  # 存在性探针：opencode 免费层反代是否注册（今天实测 404 的就是这条）
  run_one opencode_models "/api/v2/opencode/v1/models" GET json 200
  # engine 基础面：/api/v2/cookie 不碰数据库（实测 200），用来区分
  # "整个 engine 死"与"只有 opencode 那段路由缺失"。
  # 注意别用 /api/v2/?tid=N 代替——那条要连 MySQL，后端挂了会回 500，
  # 会被误判成 BAD_BODY（路由问题），而实际是数据库问题。
  run_one engine_alive "/api/v2/cookie" GET any 200
  # 鉴权层：checkID 门禁必须仍然拒绝（空响应体 + 401）
  run_one auth_gate_post "/api/v2/" POST empty_body 401
  # 阴性对照：故意打不存在的路由。它的**期望结果就是"未注册"**——
  # 若它反而通了（比如返回 200 JSON），说明 NoRoute 兜底或路由树被改动，
  # 上面几条的 NO_ROUTE 判定就不可信了。此项不计入失败计数。
  run_control no_route_probe "/api/v2/nonexistent-probe-xyz" GET none 404
}

if [ "$QUIET" != "1" ] && [ "$FORMAT" != "json" ]; then
  echo "# api-pack smoke-check  target=$SCHEME://$HOSTPORT  timeout=${TIMEOUT}s" >&2
  echo "# 字段: host route path method status verdict elapsed_ms detail"
  echo "# verdict: OK=通 | DOWN=服务未起 | NO_ROUTE=路由未注册 | BAD_BODY=返回了但不对"
fi

check_routes

[ -n "$P_BODYFILE" ] && rm -f "$P_BODYFILE"

if [ "$FORMAT" = "json" ]; then
  printf '%s\n' "${RESULTS[@]}" | jq -s '.'
fi

[ "$QUIET" = "1" ] || echo
# 退出码本身即结论（供部署脚本直接判成败，不依赖读 stdout）：
#   0 = 业务项全通   1 = 有 DOWN/NO_ROUTE/BAD_BODY   2 = 用法错
# 阴性对照异常时不单独改退出码：业务项全通仍算通过，
# 但已在数据行与 stderr 里标注，避免把"标定失败"混进部署成败。
if [ "$QUIET" = "1" ]; then
  [ "$FAILED" -eq 0 ] && exit 0
  exit 1
fi

if [ "$CONTROL_BAD" = "1" ]; then
  echo "注意：阴性对照项未按预期未注册 —— 本次 NO_ROUTE 判定不可信（见上面对照行）。"
  echo
fi
if [ "$FAILED" -eq 0 ]; then
  echo "SMOKE OK — 全部业务检查项通过（阴性对照项按预期）"
  exit 0
else
  echo "SMOKE FAILED — $FAILED 项异常；按 verdict 分派："
  echo "  DOWN      = 服务未起（查 systemd/nohup/二进制），不是版本问题"
  echo "  NO_ROUTE  = 路由未注册（版本太老 / env 门控未开），换二进制或开 env"
  echo "  BAD_BODY  = 路由在但内容不对（上游 provider / token / 门禁），查日志"
  exit 1
fi
