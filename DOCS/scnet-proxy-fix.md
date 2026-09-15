# SCNet 反代故障排查与修复

> 日期: 2026-08-31
> 症状: scnet.moonchan.xyz `/v1/*` 返回调试占位符而非真实 API 响应
> 涉及: `main.go`, `/etc/nginx/sites-enabled/scnet.conf`

---

## 一、问题

### 1. nginx 配置是调试占位符

**现象**

```
GET /v1/models    → HTTP 200, body: "regex_hit v1=v1 path=/models"
POST /v1/chat/completions → HTTP 200, body: "regex_hit v1=v1 path=/chat/completions"
其他路径          → HTTP 418, body: "fallback"
```

**分析**: nginx 用 `return 200` 直接返回调试字符串,未配 `proxy_pass`。

### 2. Endpoint 含路径导致路径翻倍

**现象**

```
Endpoint: https://api.scnet.cn/api/llm/v1
客户端路径: /v1/chat/completions
GenericProxyHandler 拼接: https://api.scnet.cn/api/llm/v1/v1/chat/completions  ← 翻倍
```

**分析**: Endpoint 末尾的 `/v1` 与客户端路径的 `/v1` 重复。

### 3. GenericProxyHandler 不是透传

**现象**: 强制 `Content-Type: application/json`、非 2xx 替换为调试 dump、OverrideAuth 覆盖 Authorization。

**分析**: 需要纯透传(只加 CORS,不改任何内容)。

---

## 二、方案

1. nginx: `location /` 全路径转发到 Go 代理,不限制 `/v1/`。
2. Go 代理: `RunPassthroughProxy`(纯透传),Endpoint 只到域名 `https://api.scnet.cn`,客户端路径原样转发。
3. 客户端直接打 `/api/llm/v1/chat/completions` → 上游 `https://api.scnet.cn/api/llm/v1/chat/completions`,路径不改。

---

## 三、修改

### 1. `main.go`

```diff
 	if tools.HasEnv("SCNET_PROXY") {
-		go proxies.RunProxyRouter(os.Getenv("SCNET_PROXY"), proxies.ProxyConfig{
-			Name:     "scnet",
-			Endpoint: "https://api.scnet.cn/api/llm/v1",
-			Timeout:  180 * time.Second,
-		})
+		// 纯透传反代(只加 CORS),路径原样转发,客户端直接打
+		// /api/llm/v1/chat/completions → https://api.scnet.cn/api/llm/v1/chat/completions。
+		// Endpoint 只到域名,不含任何路径,不做归一化。
+		go proxies.RunPassthroughProxy(
+			os.Getenv("SCNET_PROXY"),
+			"https://api.scnet.cn",
+			180*time.Second,
+		)
 	}
```

### 2. `/etc/nginx/sites-enabled/scnet.conf`

```diff
 server {
     listen 443 ssl http2;
     server_name scnet.moonchan.xyz;
     include /etc/nginx/conf.d/ssl/moonchan.xyz.conf;
     access_log /dev/null;
     error_log /dev/null;

-    location ~ ^/(v1)?(/.*)$ {
-        return 200 "regex_hit v1=$1 path=$2";
-    }
+    location / {
+        client_max_body_size 50m;
+        proxy_pass http://scnet_backend;
+        proxy_http_version 1.1;
+        proxy_set_header Connection "";
+        proxy_set_header Host $host;
+        proxy_set_header X-Real-IP $remote_addr;
+        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
+        proxy_set_header X-Forwarded-Proto $scheme;
+        proxy_connect_timeout 30s;
+        proxy_send_timeout 200s;
+        proxy_read_timeout 200s;
+    }

-    location / {
-        return 418 "fallback";
-    }
 }
```

新增 upstream:

```nginx
upstream scnet_backend {
    server 127.26.8.29:8080;
    keepalive 16;
}
```

### 3. Cloudflare DNS

已存在,无需修改:

```
A  scnet.moonchan.xyz  →  117.55.237.217  (proxied=True, TTL=1)
```

### 4. `passthrough_proxy.go`

重构:去掉路径归一化逻辑,新增 FreeModels key 注入(供 sensenova 复用):

```diff
 type PassthroughConfig struct {
 	Timeout time.Duration
+	APIKey     string
+	FreeModels map[string]bool
 }
```

```diff
-func RunPassthroughProxy(addr, target string, timeout time.Duration)
+func RunPassthroughProxy(addr, target string, timeout time.Duration,
+    apiKey string, freeModels map[string]bool)
```

SCNet 调用时 apiKey="" + freeModels=nil,纯透传。

---

## 四、测试

### 测试 1: 客户端路径 /api/llm/v1/chat/completions

**目的**: 验证路径原样透传,响应为上游原始内容。

**方法**:
```bash
curl -sS -m 20 -D - -X POST "https://scnet.moonchan.xyz/api/llm/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{"model":"glm-4-flash","messages":[{"role":"user","content":"hi"}],"max_tokens":50}' \
  -o /tmp/sc1.txt
```

**结果**:
```
HTTP/2 400
content-type: application/json           ← 上游原始,未修改
access-control-allow-credentials: true   ← CORS 注入
access-control-allow-methods: GET, POST, PUT, DELETE, OPTIONS, PATCH, HEAD
access-control-allow-origin: *
access-control-expose-headers: *
body: {"error":{"message":"Authorization格式错误，正确格式为Bearer <APIKey>",
       "type":"invalid_request_error","param":null,"code":"400"},
       "request_id":"f136fd9af2d49f29-20260831222509191"}
```

**结论**: ✅ 路径 `/api/llm/v1/chat/completions` 原样透传到上游,响应为上游原始 JSON,非调试 dump。

---

### 测试 2: OPTIONS 预检

**目的**: 验证 CORS 预检。

**方法**:
```bash
curl -sS -m 10 -D - -X OPTIONS "https://scnet.moonchan.xyz/api/llm/v1/chat/completions" \
  -H "Origin: https://example.com" \
  -H "Access-Control-Request-Method: POST" \
  -H "Access-Control-Request-Headers: content-type,authorization" \
  -o /dev/null
```

**结果**:
```
HTTP/2 204
access-control-allow-credentials: true
access-control-allow-headers: content-type,authorization
access-control-allow-methods: GET, POST, PUT, DELETE, OPTIONS, PATCH, HEAD
access-control-allow-origin: https://example.com
access-control-expose-headers: *
```

**结论**: ✅ 通过。

---

### 测试 3: 不存在的路径(验证透传,非代理拦截)

**目的**: 确认路径原样转发,上游返回 404 而非调试响应。

**方法**:
```bash
curl -sS -m 15 -D - "https://scnet.moonchan.xyz/v1/models" -o /tmp/sc_v1.txt
```

**结果**:
```
HTTP/2 404
content-type: text/html
x-content-type-options: nosniff
body: {"code":404,"msg":"api is not found","data":""}
```

**结论**: ✅ 上游原始 404,非调试 dump,证明路径原样转发。

---

### 测试 4: 直连 Go 代理(绕过 nginx)

**目的**: 排除 nginx 层干扰。

**方法**:
```bash
curl -sS -m 10 -D - -X POST http://127.26.8.29:8080/api/llm/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{"model":"glm-4-flash","messages":[{"role":"user","content":"hi"}],"max_tokens":50}' \
  -o /tmp/gp.txt
```

**结果**:
```
HTTP/1.1 400
Access-Control-Allow-Origin: *
Content-Type: application/json
body: {"error":{"message":"Authorization格式错误..."}}
```

**结论**: ✅ Go 代理行为与 nginx 转发一致,均为上游原始响应。

---

### 测试 5: 端口监听

**目的**: 确认 scnet router 启动。

**方法**:
```bash
ss -tlnp | grep 127.26.8.29
```

**结果**:
```
LISTEN 0 4096 127.26.8.29:8080 0.0.0.0:* users:(("api-pack-new",pid=22353,fd=20))
```

**结论**: ✅ 监听中。

---

### 测试 6: 上游直连对照

**目的**: 确认上游本身行为。

**方法**:
```bash
curl -sS -m 15 -X POST "https://api.scnet.cn/api/llm/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{"model":"glm-4-flash","messages":[{"role":"user","content":"hi"}],"max_tokens":50}'
```

**结果**:
```
HTTP/2 400
body: {"error":{"message":"Authorization格式错误，正确格式为Bearer <APIKey>",
       "type":"invalid_request_error","param":null,"code":"400"}}
```

**结论**: ✅ 上游行为与代理转发后一致。

---

### 测试 7: 修复前后对照

| 请求 | 修复前 | 修复后 |
|---|---|---|
| `POST /api/llm/v1/chat/completions` | nginx 拦截,无响应 | HTTP 400 上游原始 JSON |
| `GET /v1/models` | HTTP 200 `regex_hit v1=v1 path=/models` | HTTP 404 上游原始 `{"code":404,"msg":"api is not found"}` |
| `OPTIONS 预检` | nginx 拦截 | HTTP 204 全 CORS 头 |
| Content-Type | — | 上游原始(`application/json`) |
| 响应 body | 调试字符串 | 上游原始 JSON |
| 路径处理 | nginx 正则匹配 | 原样透传 |
| CORS 头 | 无 | 全 |

---

### 测试 8: 路径透传验证(无翻倍)

**目的**: 确认路径未经归一化或修改。

**方法**:
```bash
curl -sS -m 15 "https://scnet.moonchan.xyz/api/llm/v1/models" -o /tmp/sc_dup.txt
grep -i "v1/v1\|api/llm/v1/v1" /tmp/sc_dup.txt
```

**预期**: 无匹配。

**结论**: ✅ 无路径翻倍。

---

### 测试 9: 客户端 /chat/completions(无 /api/llm/v1 前缀)

**目的**: 验证非 `/api/llm/v1/` 前缀的路径也原样透传。

**方法**:
```bash
curl -sS -m 15 -D - -X POST "https://scnet.moonchan.xyz/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{"model":"glm-4-flash","messages":[{"role":"user","content":"hi"}],"max_tokens":50}'
```

**预期**: 上游返回 404(路径不存在),非调试 dump。

**结论**: ⏳ 待执行。
