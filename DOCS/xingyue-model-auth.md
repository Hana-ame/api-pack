# xingyue 代理：入站 auth 检查 + 按模型注入不同出站 token（cloudcone 部署核对单）

状态：**代码完成、测试通过，未部署**。部署前按本文与用户核对。

## 功能改动

| 文件 | 内容 |
|---|---|
| `internal/proxy/inbound_auth.go` | 新增。前置入站 auth 检查中间件（单人自用门禁）：Authorization 携带 env 里的正确入站 token（Bearer，容错裸 token / scheme 大小写）才放行，否则 401 不碰上游；OPTIONS 放行；**空配置恒拒绝**（配置丢失不会变成全放通） |
| `internal/proxy/model_tokens.go` | 新增。ModelTokenTable：按 env spec 从 JSON 加载「模型 → 出站 token 组」；组内多 token 逐次轮转；`*` 键兜底；文件形态 mtime/大小变化 5s 内热重载（轮换 token 不重启）；日志绝不输出 token 值 |
| `pkg/jsonpath/` | 新增。零依赖 JSONPath 子集求值器（`$` `.k` `['k']` `[n]` 负下标 `[*]` `..k` 递归），带测试 |
| `internal/proxy/generic_proxy.go` | ProxyConfig 增加 `InboundToken` / `ModelTokens`；handler 先剥离门禁 token（绝不带给上游）→ 命中模型表则强制覆盖 Authorization → 未命中走原逻辑；`RunProxyRouter` 在 `InboundToken` 非空时装中间件（空=旧行为不变，其他代理零影响） |
| `internal/app/app.go` | xingyue 块接线三个新 env；token 表加载失败只跳过 xingyue 本代理并 ERROR 日志（宁可不启动也不带错 key，进程其他服务不受影响） |
| 测试 | `model_tokens_test.go` `xingyue_flow_test.go` `jsonpath_test.go`：门禁放行/拒绝、按模型覆盖、组轮转、兜底键、GET 无体透传、门禁 token 不外泄、热重载、坏文件保留旧表、错误 spec 拒载 |

**只影响 xingyue 一个代理**：其余代理（groq/siliconflow/gemini/sensenova/scnet…）不装这两个字段，行为逐字节不变（回归用例含旧 FreeModels 注入测试）。

## env（cloudcone `/root/.env` 追加，现无任何 XINGYUE_* 条目）

```dotenv
# xingyue 反代: 监听地址(开关) + 单人入站 token + 出站 model token 组
XINGYUE_PROXY=127.26.8.31:8080
XINGYUE_INBOUND_TOKEN=<随机串, 核对时生成给你>
XINGYUE_MODEL_TOKENS=/root/.secrets/xingyue-tokens.json#$.xingyue.tokens
```

- `XINGYUE_PROXY`：`127.26.8.31:8080` 已实测空闲（现占用 .2/.10/.12/.13/.23/.25/.29）。
- `XINGYUE_MODEL_TOKENS` spec = `<source>[#<jsonpath>]`；source 也可写 `env://VAR_NAME`（JSON 内联在另一个 env 变量里）。jsonpath 指到「模型→token」对象即可，token 值形态：字符串（单 token）或字符串数组（组内轮转），`"*"` 键为兜底。
- token JSON（**内容等你提供**）结构示例：

```json
{
  "xingyue": {
    "tokens": {
      "some-model-a": "sk-outbound-a",
      "some-model-b": ["sk-b-1", "sk-b-2"],
      "*": "sk-default"
    }
  }
}
```

落到 cloudcone：`/root/.secrets/xingyue-tokens.json`，`chmod 600`（root only）。改这个文件 5 秒内生效，无需重启。

## nginx（cloudcone 新增 `/etc/nginx/sites-available/xingyue.conf`，仿 scnet.conf）

```nginx
upstream xingyue_backend {
    server 127.26.8.31:8080;
    keepalive 16;
}

server {
    listen 443 ssl http2;
    server_name xingyue.moonchan.xyz;

    include /etc/nginx/conf.d/ssl/moonchan.xyz.conf;

    access_log /dev/null;
    error_log /dev/null;

    location / {
        client_max_body_size 50m;
        proxy_pass http://xingyue_backend;
        proxy_http_version 1.1;
        proxy_set_header Connection "";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_connect_timeout 30s;
        proxy_send_timeout 200s;
        proxy_read_timeout 200s;   # SSE 流式: 与 scnet 对齐
    }
}
```

现状实测：`xingyue.moonchan.xyz` 已在 Cloudflare 解析（橙云），但两台的 nginx 都没有该 server 块——现在落到通配站（返回静态目录页）。加上此 conf 后即接管。**注意**：CF 记录的源站 IP 归属未知（vps/cloudcone 都直接响应 200），部署后会用真域名做端到端验证；若不指向 cloudcone，需要你在 Cloudflare 后台把该记录源站改到 cloudcone（117.55.237.217）。

## 部署步骤（确认后执行；生产文件先备份）

1. 本地 `bash build.sh` 的构建段：`CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o api-pack-new ./cmd/api-pack`（cloudcone 实测 x86_64）。
2. `scp api-pack-new root@cloudcone:~/temp`（沿用 build.sh 的 `~/script/scp.sh` 通道；**不碰 vps**）。
3. cloudcone 上：
   - `cp /root/api-pack-new /root/api-pack-new.bak.$(date +%s)`（沿用现有 bak 命名惯例）
   - `cp /root/.env /root/.env.bak.$(date +%s)` ← 上次生产事故就是 .env 被覆盖，必须先备份
   - 追加 3 行 env（append，绝不重写整个 .env）
   - 写 `/root/.secrets/xingyue-tokens.json`（内容 = 你给的 JSON）+ chmod 600
   - 落 nginx conf → `ln -s ../sites-available/xingyue.conf sites-enabled/` → `nginx -t && systemctl reload nginx`
4. 换二进制：`pkill api-pack-new; mv temp api-pack-new; chmod +x; cd /root && nohup ./api-pack-new >> /root/api-pack.log 2>&1 &`（现有进程就是 nohup 方式，无 systemd）。
5. 验证（全部通过才算完成）：
   - `ss -ltn` 见 `127.26.8.31:8080`
   - 无 token → 401；错 token → 401；对 token + 表内模型 → 上游 200 且错误回显里 `Authorization: ***`
   - GET /v1/models 带门禁 token → 上游不收到任何 Authorization（用日志/回显核验）
   - 真域名 `https://xingyue.moonchan.xyz/v1/chat/completions` 带门禁 token 走一小段流式 → 200 + SSE；若域名仍回静态目录 → CF 源站未指过来，报给你处理
   - 其他服务不受影响：scnet/sensenova/echo 等入口各打一枪
6. 回滚预案：恢复 bak 二进制 + nohup 重启即回到旧行为（无 xingyue）。nginx 侧删软链 reload 即可。

## 待你核对/提供的点

1. env 名与 spec 格式、`127.26.8.31:8080` 地址、域名 `xingyue.moonchan.xyz`（nginx server_name）是否 OK。
2. 入站 token：我在你点头后生成 `openssl rand -hex 24` 给你（或你指定值）。
3. 出站 model token JSON：发给我（模型名 → token/数组，含是否需要 `"*"` 兜底），以及你希望的 JSONPath（默认我就按 `$.xingyue.tokens` 组织文件）。
4. 门禁语义确认：**XINGYUE_INBOUND_TOKEN 未设时 = 关闭检查**（保持别的机器旧行为）；若你要「设了监听就必须带 token」的 fail-closed 变体，说一声，改一行门控。
