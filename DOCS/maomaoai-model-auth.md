# maomaoai 代理：入站 auth 检查 + 按模型注入不同出站 token（cloudcone 部署核对单）

状态：**代码完成、本地冒烟矩阵全绿，未部署**。机制与 xingyue 完全同款（复用 `inbound_auth.go` / `model_tokens.go` / `pkg/jsonpath` / `generic_proxy.go`，见 `xingyue-model-auth.md`），本次净新增只有 `internal/app/app.go` 的 maomaoai 块。

## 功能改动

| 文件 | 内容 |
|---|---|
| `internal/app/app.go` | 新增 maomaoai 块：`startIfEnv("MAOMAOAI_PROXY", …)`，接 `MAOMAOAI_INBOUND_TOKEN` + `MAOMAOAI_MODEL_TOKENS`；fail-closed 同 xingyue（没给入站 token 则该代理不启动，不影响进程其他服务） |
| 其余机制文件 | **零改动**，与 xingyue 共用 |

上游：`https://api.maomaoai.pro`（New API 网关）。Endpoint 不含 `/v1`，客户端 `/v1/chat/completions` 原样透传为 `https://api.maomaoai.pro/v1/chat/completions`。

## 本地冒烟实测（2026-09-15，构建自当前工作树）

| 格 | 期待 | 实测 |
|---|---|---|
| 设了 `MAOMAOAI_PROXY` 没给 `MAOMAOAI_INBOUND_TOKEN` | ERROR 日志 + 不监听 | ✓ `maomaoai 代理不启动(fail-closed)`，18080 未听 |
| 无 token POST | 401，不碰上游 | ✓ 网关自产 body `invalid or missing inbound token` |
| 错 token POST | 401 | ✓ 同上 |
| 裸 token（无 `Bearer ` 前缀） | 容错放行 | ✓ 打到上游（fake key → `new_api_error Invalid token`，证明确实出站且注入生效） |
| Bearer token + 表内模型 | 注入表内 token 覆盖出站 | ✓ 上游只见 `sk-fake-*`；回显 `Authorization: ***` 打码，门禁 token 未透传 |
| OPTIONS 预检 | 204 | ✓ |
| token 表加载日志 | 只列模型名不列值 | ✓ `models=[* deepseek-v4.1-flash qwen3.8-flash]` |

单元测试 `go test ./internal/proxy/ ./pkg/jsonpath/` 全过；`go build ./...` exit 0。

## env（cloudcone `/root/.env` 追加，现无任何 MAOMAOAI_* 条目——实测 2026-09-15）

```dotenv
# maomaoai 反代: 监听地址(开关) + 单人入站 token + 出站 model token 组
MAOMAOAI_PROXY=127.26.8.32:8080
MAOMAOAI_INBOUND_TOKEN=<随机串, 核对时生成给你>
MAOMAOAI_MODEL_TOKENS=/root/.secrets/maomaoai-tokens.json#$.maomaoai.tokens
```

- `127.26.8.32:8080`：2026-09-15 实测空闲（现占用 .2/.10/.12/.13/.23/.25/.29/.31）。
- token JSON（**内容等你提供**）：

```json
{
  "maomaoai": {
    "tokens": {
      "deepseek-v4.1-flash": "sk-上游真key",
      "qwen3.8-flash": ["sk-真key-1", "sk-真key-2"],
      "*": "sk-兜底"
    }
  }
}
```

落到 `/root/.secrets/maomaoai-tokens.json`，`chmod 600`；改文件 5 秒内热生效，不重启。模型名精确匹配（大小写敏感）；`"*"` 兜底可把 21 个上游模型全放开或全指同一 key。

## nginx（cloudcone 新增 `/etc/nginx/sites-available/maomaoai.conf`，仿 xingyue.conf）

```nginx
upstream maomaoai_backend {
    server 127.26.8.32:8080;
    keepalive 16;
}

server {
    listen 443 ssl http2;
    server_name maomaoai.moonchan.xyz;

    include /etc/nginx/conf.d/ssl/moonchan.xyz.conf;

    access_log /dev/null;
    error_log /dev/null;

    location / {
        client_max_body_size 50m;
        proxy_pass http://maomaoai_backend;
        proxy_http_version 1.1;
        proxy_set_header Connection "";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_connect_timeout 30s;
        proxy_send_timeout 200s;
        proxy_read_timeout 200s;   # SSE 流式
    }
}
```

域名暂定 `maomaoai.moonchan.xyz`（与 provider id 一致；要短名 `maomao.moonchan.xyz` 说一声）。**待部署时实测**：Cloudflare 是否已有该记录/通配记录；若没有需你先加记录（橙云），源站指向 cloudcone。

## 部署步骤（确认后执行；全程按五段 SOP：构建核对 → 上传+双备份 → env 只 append + diff 验证 → nginx+换二进制 → 验证/回滚预案）

1. 本地：`CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o api-pack-new ./cmd/api-pack`，记录 md5。
2. scp 到 cloudcone `~/temp`，md5 比对一致；`.env` 与运行中二进制各做时间戳备份。
3. append 上面 3 行 env（**只 append**，diff 期待形如 `N,M aN+K` 且零 `<` 行）；写 tokens JSON + chmod 600 + `python3 -m json.tool` 校验。
4. nginx conf → 软链 sites-enabled → `nginx -t` → reload；`pkill api-pack-new` → 换二进制 → nohup 拉起 → `ss -ltn` 见 `127.26.8.32:8080`、日志无 ERROR。
5. 验证矩阵：门禁 401（无/错 token）、对 token 打一枪真模型 → 200 + SSE、回显 `Authorization: ***`、`xingyue.moonchan.xyz` 等同机服务回归各打一枪、真域名端到端。
6. 回滚：还原 bak 二进制 + nohup；env 删追加行（备份在案）；nginx 删软链 reload。新块 fail-closed，任何一步失败都不裸奔。

## DSH 侧联动（反代上线并端到端验证后，可选，你拍板）

真正把 DSH 的 `maomaoai` provider 也"像 xingyue 一样"走自家域：
1. `~/.dsh/settings.yaml` 的 `maomaoai.baseURL` → `https://maomaoai.moonchan.xyz/v1`（仍必须带 `/v1`）。
2. `~/.dsh/.credentials.yaml` 的 `MAOMAOAI_API_KEY` 值 → 换成门禁 token（真 key 从此只存在于 cloudcone 的 tokens JSON）。
3. ⚠️ 当前默认路由就是 `maomaoai/deepseek-v4.1-flash`（本机主力），换 baseURL 前先确认反代链路稳定，出问题回滚 settings 两处即可。

## 待你核对/提供的点

1. 监听 `127.26.8.32:8080`、域名 `maomaoai.moonchan.xyz`、env 三名、JSONPath `$.maomaoai.tokens` 是否 OK。
2. 入站 token：点头后我生成 `openssl rand -hex 24` 给你（或你指定）。
3. 出站 model token JSON：把 api.maomaoai.pro 的真 key 给我（模型 → key/数组，是否带 `"*"` 兜底）；若就一个 key 全模型通用，JSON 只写 `"*"` 一条即可。
4. DSH 侧要不要切到自家域（上面联动节），还是仅对外共享用。
