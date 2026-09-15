# SenseNova 反代故障排查与修复

> 日期: 2026-08-30 ~ 2026-08-31
> 症状: sensenova.moonchan.xyz `/v1/*` 全部 502 / 400 `{"error":"invalid JSON"}`
> 涉及: `main.go`, `proxies/passthrough_proxy.go`, `/etc/nginx/sites-enabled/sensenova.conf`

---

## 一、问题

### 1. SENSENOVA_PROXY 被注释 → router 未启动 → nginx 502

**现象**:
```
GET /v1/models           → HTTP 502, body: "error code: 502"
OPTIONS 预检             → HTTP 502 (非 204)
无 Access-Control-Allow-Origin 头
```

**分析**: `.env` 中 SENSENOVA_PROXY 被 `#` 注释 → `tools.HasEnv()` 判空 → router 未启动 → nginx 转发到死端口 → 502。

### 2. GenericProxyHandler 不是透传

**现象**:
```
GET /v1/models           → HTTP 400 {"error":"invalid JSON"}    ← 强制解析 body
GET /v1/nope             → HTTP 400 {"error":"invalid JSON"}    ← 同上
非 2xx 响应              → [Request Headers]\n... 调试 dump     ← 非上游原始
Content-Type             → application/json; charset=utf-8       ← 强制改写
```

**分析**: `RunProxyRouter`(GenericProxyHandler)强制解析 body 取 model 字段,GET 无 body 时返回 400,非 2xx 时替换为调试 dump。

### 3. 上游 token.sensenova.cn CORS 预检缺失

**现象**:
```
OPTIONS /v1/images/generations (Origin: sensenova.moonchan.xyz)
→ 404, 缺 Access-Control-Allow-Methods / Allow-Headers
```

**分析**: 上游官方 CORS 预检不完整,浏览器直连被拦,反代必须补齐 CORS 头。

### 4. `:=` 遮蔽导致 key 注入失效

**现象**: FreeModels 模式下服务端 key 从未注入,上游 401。

**分析**: `:=` 在 if 块内新建内层 model,外层 model 恒为 `""` → `FreeModels[""]` 恒 false。

---

## 二、方案

### 问题 1: 取消注释 .env
```bash
sed -i 's/^#SENSENOVA_PROXY/SENSENOVA_PROXY/' /root/.env
sed -i 's/^#SENSENOVA_API_KEY/SENSENOVA_API_KEY/' /root/.env
pkill -f api-pack-new; sleep 2; cd /root && nohup ./api-pack-new > ./nohup.out 2>&1 &
```

### 问题 2: 改用 PassthroughProxy
`RunProxyRouter` → `RunPassthroughProxy`,透传路径 + 响应 + Content-Type,仅加 CORS + FreeModels key 注入。

### 问题 3: CORS 由代理补齐
CORSMiddleware 注入 `Access-Control-Allow-*`,OPTIONS 返回 204。

### 问题 4: 已修复
commit `66ff928`,附回归测试。

---

## 三、修改

### 1. `proxies/passthrough_proxy.go`

```diff
 type PassthroughConfig struct {
 	Timeout time.Duration
+	// APIKey 非空 + FreeModels 非空时, 请求体 model 命中 FreeModels 且客户端
+	// 未带 Authorization 时, 注入 "Bearer <APIKey>"。否则纯透传。
+	APIKey     string
+	FreeModels map[string]bool
 }
```

```diff
-// 路径归一化逻辑(去掉)
-	base := strings.TrimRight(target, "/")
-	prefix := ""
-	if strings.HasSuffix(base, "/v1") {
-		base = strings.TrimSuffix(base, "/v1")
-		prefix = "/v1"
-	}
+	base := strings.TrimRight(target, "/")
+
+	return func(c *gin.Context) {
+		body, err := io.ReadAll(c.Request.Body)
+		...
+		targetURL := base + c.Request.URL.Path    // 路径原样转发
+		...
+		// FreeModels key 注入
+		if cfg.APIKey != "" && len(cfg.FreeModels) > 0 {
+			auth := c.Request.Header.Get("Authorization")
+			if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
+				model, err := extractModel(body)
+				if err == nil && cfg.FreeModels[model] {
+					req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
+				}
+			}
+		}
+		...
+	}
```

```diff
-func RunPassthroughProxy(addr, target string, timeout time.Duration)
+func RunPassthroughProxy(addr, target string, timeout time.Duration,
+    apiKey string, freeModels map[string]bool)
```

### 2. `main.go`

```diff
 	if tools.HasEnv("SENSENOVA_PROXY") {
-		go proxies.RunProxyRouter(os.Getenv("SENSENOVA_PROXY"), proxies.ProxyConfig{
-			Name:     "sensenova",
-			Endpoint: "https://token.sensenova.cn",
-			APIKey:   os.Getenv("SENSENOVA_API_KEY"),
-			FreeModels: map[string]bool{
-				"sensenova-u1.5-lite": true,
-				"sensenova-u1-fast":   true,
-			},
-			Timeout: 180 * time.Second,
-			MaskedHeaders: []string{"Authorization", "X-Api-Key", "Cookie"},
-		})
+		go proxies.RunPassthroughProxy(
+			os.Getenv("SENSENOVA_PROXY"),
+			"https://token.sensenova.cn",
+			180*time.Second,
+			os.Getenv("SENSENOVA_API_KEY"),
+			map[string]bool{
+				"sensenova-u1.5-lite": true,
+				"sensenova-u1-fast":   true,
+			},
+		)
 	}
```

### 3. `/root/.env`

```diff
-#SENSENOVA_PROXY=127.26.8.25:8080
-#SENSENOVA_API_KEY=sk-Ji81k...
+SENSENOVA_PROXY=127.26.8.25:8080
+SENSENOVA_API_KEY=sk-Ji81k...
```

### 4. nginx

无修改(配置正确):
```nginx
location /v1/ { proxy_pass http://127.26.8.25:8080; ... }
location / { root /var/www/sensenova; try_files $uri $uri/ /index.html; }
```

---

## 四、测试

### 测试 1: GET /v1/models(无 body)

**目的**: 验证路径透传,上游原始响应(非调试 dump)。

**方法**:
```bash
curl -sS -m 15 -D - "https://sensenova.moonchan.xyz/v1/models"
```

**结果**:
```
HTTP/2 401
content-type: application/json
access-control-allow-origin: *
body: {"error": {"code": 16,"message": "Authorization Not Found"}}
```

**结论**: ✅ 通过 — 上游原始 JSON,非 `{"error":"invalid JSON"}`。

---

### 测试 2: GET /v1/nope(不存在的路径)

**目的**: 验证路径原样转发,上游返回 404。

**方法**:
```bash
curl -sS -m 15 -D - "https://sensenova.moonchan.xyz/v1/nope"
```

**结果**:
```
HTTP/2 404
content-type: application/json
body: {"error": {"code": 5,"message": "NOT_FOUND","details": []}}
```

**结论**: ✅ 通过 — 上游原始 404。

---

### 测试 3: OPTIONS 预检

**目的**: 验证 CORS 预检。

**方法**:
```bash
curl -sS -m 10 -D - -X OPTIONS "https://sensenova.moonchan.xyz/v1/images/generations" \
  -H "Origin: https://sensenova.moonchan.xyz" \
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
access-control-allow-origin: https://sensenova.moonchan.xyz
access-control-expose-headers: *
```

**结论**: ✅ 通过。

---

### 测试 4: POST /v1/images/generations(免费模型, 无 key)

**目的**: 验证 FreeModels key 注入生效 + 透传响应。

**方法**:
```bash
curl -sS -m 120 -D - -X POST "https://sensenova.moonchan.xyz/v1/images/generations" \
  -H "Content-Type: application/json" \
  -d '{"model":"sensenova-u1.5-lite","prompt":"a cat","size":"512x512","steps":4}' \
  -o /tmp/sn_gen.txt
```

**结果**:
```
HTTP/2 200
content-type: application/json; charset=utf-8
x-request-id: 75488f99-370f-490b-90f5-022c408e6b27
access-control-allow-origin: *
body: {"created":1788188393,"data":[{"b64_json":"iVBORw0..."}]}
耗时: 25.5s
```

**结论**: ✅ 通过 — 200,客户端无 key 但 FreeModels key 注入生效,上游正常生图。

---

### 测试 5: POST /v1/images/generations(客户端自带 key)

**目的**: 验证客户端自带 Bearer 时原样透传。

**方法**:
```bash
curl -sS -m 30 -X POST "https://sensenova.moonchan.xyz/v1/images/generations" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-client-key" \
  -d '{"model":"sensenova-u1.5-lite","prompt":"x","size":"512x512","steps":2}'
```

**预期**: 上游返回客户端 key 对应的响应(或鉴权错误)。

**结论**: ⏳ 待有效 key 后执行。

---

### 测试 6: POST /v1/images/generations(非免费模型)

**目的**: 验证非 FreeModels 模型不注入 key。

**方法**:
```bash
curl -sS -m 15 -D - -X POST "https://sensenova.moonchan.xyz/v1/images/generations" \
  -H "Content-Type: application/json" \
  -d '{"model":"sensenova-u1.5","prompt":"x","size":"512x512","steps":2}' \
  -o /tmp/sn_paid.txt
```

**预期**: HTTP 401 `Authorization Not Found`(不注入 key)。

---

### 测试 7: 上游直连对照

**目的**: 确认上游本身行为。

**方法**:
```bash
curl -sS -m 15 "https://token.sensenova.cn/v1/models"
```

**结果**:
```
HTTP/1.1 401
body: {"error": {"code": 16,"message": "Authorization Not Found"}}
```

**结论**: ✅ 上游行为与代理转发后一致。

---

### 测试 8: 修复前后对照

| 请求 | 修复前 | 修复后 |
|---|---|---|
| `GET /v1/models` | 502 `error code: 502` → 400 `{"error":"invalid JSON"}` | 401 `{"error":{"code":16,...}}` |
| `GET /v1/nope` | 502 | 404 `{"error":{"code":5,"message":"NOT_FOUND"}}` |
| `POST /v1/images/generations` | 502 | 200 上游原始 |
| OPTIONS 预检 | 502 | 204 全 CORS 头 |
| Content-Type | 强制 `application/json` | 上游原始 |
| 非 2xx 响应 | 调试 dump | 上游原始 JSON |
| 生图(免费模型,无 key) | 502 | 200 ✅ key 注入 |

---

### 测试 9: api-pack 端口监听

**方法**:
```bash
ss -tlnp | grep 127.26.8.25
```

**结果**:
```
LISTEN 0 4096 127.26.8.25:8080 0.0.0.0:* users:(("api-pack-new",pid=22685,fd=17))
```

**结论**: ✅ 监听中。
