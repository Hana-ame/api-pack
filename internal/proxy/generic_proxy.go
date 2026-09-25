package proxy

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	middleware "github.com/Hana-ame/api-pack/pkg/ginutil/middleware"
	tools "github.com/Hana-ame/api-pack/pkg/utils"
	"github.com/andybalholm/brotli"
	"github.com/gin-gonic/gin"
	"github.com/klauspost/compress/zstd"
)

// ProxyConfig defines the configuration for a single proxy target
type ProxyConfig struct {
	Name          string
	Endpoint      string
	APIKey        string
	FreeModels    map[string]bool
	FreeModelsAll bool
	Models        map[string]ModelInfo
	CustomHeaders map[string]string
	// OverrideAuth 为 true 时, 上游 Authorization 始终用 APIKey 覆盖客户端传入值
	// (代理自带 key, 前端无需持有真实 key)。key 必须来自 .env。
	OverrideAuth bool
	// InboundToken 非空时启用【前置入站 auth 检查】(单人自用, RunProxyRouter 装载
	// InboundAuthMiddleware): Authorization 必须携带该 env 里的正确入站 token, 否则 401。
	// 入站 token 是本网关的门禁凭据, 对上游无意义, 命中后绝不透传给上游(见 handler 剥离)。
	InboundToken string
	// ModelTokens 非 nil 时启用【按模型注入不同 auth token override】:
	// 请求体 model 命中表(含 "*" 兜底键) → 上游 Authorization 强制改写为该模型的
	// token(覆盖客户端传入值, 组内多 token 逐次轮转); 未命中 → 走原有透传逻辑。
	ModelTokens *ModelTokenTable
	// Timeout > 0 时为该代理单独设置上游超时, 覆盖 http.DefaultClient 的 30s 默认
	// (生图等同步接口可能需数十秒)。
	Timeout time.Duration
	// MaskedHeaders 列出错误(非 2xx)调试回显里需要用 *** 屏蔽的 header key(大小写不敏感)。
	// 防止 Authorization / X-Api-Key 等敏感信息(尤其服务端注入的 APIKey)泄露给客户端。
	// 为空时仍会默认屏蔽 Authorization、Cookie 以及常见 key/token 头。
	MaskedHeaders []string
	// DropBodyKeys 非空时, 转发上游前从请求 JSON body 顶层删除这些键——
	// 用于 Google generativelanguage 等【严格】OpenAI 兼容端点: 它们拒绝 OpenAI
	// 专有字段(如 store / stream_options), 报 400 "Unknown name ... Cannot find field"。
	// 只删顶层键; 非 JSON 体原样透传。
	DropBodyKeys []string
}

type ModelInfo struct {
	Name string
}

// GenericProxyHandler returns a gin handler that proxies requests based on the provided config
func GenericProxyHandler(config ProxyConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "failed to read request body",
			})
			return
		}
		c.Request.Body.Close()

		var model string
		if len(config.FreeModels) > 0 || config.ModelTokens != nil {
			reqMap, err := tools.ReaderToJSON(bytes.NewReader(bodyBytes))
			if err != nil {
				// 只开了 model token 表(没开 FreeModels 白名单)时, 非 JSON 体
				// (如 GET /v1/models)跳过解析, 原样透传由上游报格式错误。
				if len(config.FreeModels) > 0 {
					c.JSON(http.StatusBadRequest, gin.H{
						"error": "invalid JSON",
					})
					return
				}
			} else {
				// 【坑】这里必须用 = 赋值给外层 model, 不能 := (会新建内层 model 遮蔽外层):
				// 曾因 := 遮蔽导致外层 model 永远是 "", FreeModels[model] 恒 false,
				// 免费模型的 APIKey 注入从未生效, 上游 401 "Authorization Not Found"。
				// 发现背景: 2026-08-25 sensenova.moonchan.xyz 生图页 E2E 调试时暴露。
				var ok bool
				model, ok = reqMap.GetOrDefault("model", "").(string)
				if !ok {
					model = ""
				}
				if len(config.FreeModels) > 0 && model == "" {
					c.JSON(http.StatusBadRequest, gin.H{
						"error": "model field is required",
					})
					return
				}
			}
		}

		// Setup headers
		headers := tools.NewHeader(c.Request.Header)
		auth := headers.Get("Authorization")

		// 入站检查开启且客户端带的就是门禁 token: 先剥离, 绝不带给上游——
		// 它是本代理自己的入站凭据, 对上游无意义, 泄露出去即外流。
		if config.InboundToken != "" && bearerTokenEquals(auth, config.InboundToken) {
			headers.Set("Authorization", "") // utils.Header: 空串 Set = Del
			auth = ""
		}

		overridden := false
		if config.ModelTokens != nil && model != "" {
			// 按模型注入不同的出站 token override: 命中即强制覆盖客户端传入值
			if tok, ok := config.ModelTokens.Token(model); ok {
				headers.Set("Authorization", "Bearer "+tok)
				overridden = true
			}
		}
		if !overridden {
			if config.OverrideAuth && config.APIKey != "" {
				// 代理自带 key: 始终用 .env 里的 APIKey 覆盖, 前端无需持有真实 key
				headers.Set("Authorization", "Bearer "+config.APIKey)
			} else if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
				if config.APIKey != "" && (config.FreeModelsAll || config.FreeModels[model]) {
					headers.Set("Authorization", "Bearer "+config.APIKey)
				}
			}
			// else 直接保留原来的 auth
		}
		headers.Set("Content-Type", "application/json")

		// 防止 http.DefaultClient 自动加 Accept-Encoding: gzip 并自动解压.
		// 上游若声明 Content-Encoding 但 body 非对应编码 (CDN 常见),
		// transport 的解压会失败并吞掉整个 body, 导致错误回显为空.
		// 客户端显式带了 Accept-Encoding 时保留原值 (它期望压缩).
		if headers.Get("Accept-Encoding") == "" {
			headers.Set("Accept-Encoding", "identity")
		}

		for k, v := range config.CustomHeaders {
			headers.Set(k, v)
		}

		// Forward request
		targetURL := config.Endpoint + c.Request.URL.Path
		if targetURL == config.Endpoint {
			// Handle cases where Endpoint might not have trailing slash and path is empty
		} else if !strings.HasSuffix(config.Endpoint, "/") && !strings.HasPrefix(c.Request.URL.Path, "/") {
			targetURL = config.Endpoint + "/" + c.Request.URL.Path
		}

		// 按配置删除请求体顶层 OpenAI 专有字段(store/stream_options 等):
		// Google 等严格 OpenAI 兼容端点会 400 "Unknown name ... Cannot find field"。
		if len(config.DropBodyKeys) > 0 {
			if cleaned, err := stripBodyKeys(bodyBytes, config.DropBodyKeys); err == nil {
				bodyBytes = cleaned
			}
			// 解析失败(非 JSON / 非对象, 如 GET 无体)保持原样透传
		}

		// 上游请求: Timeout>0 时用独立 client 覆盖默认 30s (生图同步接口可能需数十秒)
		client := http.DefaultClient
		if config.Timeout > 0 {
			cc := *http.DefaultClient
			cc.Timeout = config.Timeout
			client = &cc
		}
		req, err := http.NewRequest(c.Request.Method, targetURL, bytes.NewReader(bodyBytes))
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "create request: " + err.Error()})
			return
		}
		req.Header = headers.Header
		resp, err := client.Do(req)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "upstream request failed: " + err.Error()})
			return
		}
		defer resp.Body.Close()

		// DELETE all cf-* headers
		for k := range resp.Header {
			if strings.HasPrefix(strings.ToLower(k), "cf-") {
				resp.Header.Del(k)
			}
		}

		// Return response
		if resp.StatusCode != http.StatusOK {
			// 先读原始 body 到 buffer (resp.Body 是一次性流, 必须先消费).
			// 之前用 myfetch.ResponseToReader 直接读流, 若上游声明了 Content-Encoding
			// 但 body 并非对应编码 (CDN 常见), 解压失败会吞掉整个流, 导致回显为空.
			rawBody, readErr := io.ReadAll(resp.Body)
			bodyStr := string(rawBody)
			if readErr != nil {
				bodyStr = "[Transport Decompression Error: " + readErr.Error() + "]"
				if len(rawBody) > 0 {
					bodyStr += "\n[Partial Data: " + string(rawBody) + "]"
				}
			}

			// 从 buffer 尝试解压, 解压失败则回退原始 bytes.
			switch strings.ToLower(resp.Header.Get("Content-Encoding")) {
			case "gzip":
				if gr, err := gzip.NewReader(bytes.NewReader(rawBody)); err == nil {
					if d, dErr := io.ReadAll(gr); dErr == nil {
						bodyStr = string(d)
					}
				}
			case "br":
				if d, dErr := io.ReadAll(brotli.NewReader(bytes.NewReader(rawBody))); dErr == nil {
					bodyStr = string(d)
				}
			case "zstd":
				if zr, err := zstd.NewReader(bytes.NewReader(rawBody)); err == nil {
					if d, dErr := io.ReadAll(zr); dErr == nil {
						bodyStr = string(d)
					}
				}
			}

			// Sort and format request headers (敏感头用 *** 打码, 防 APIKey 泄露)
			var reqHeaderLines []string
			for k, v := range headers.Header {
				var vals []string
				for _, vv := range v {
					vals = append(vals, maskHeaderValue(k, vv, config.MaskedHeaders))
				}
				reqHeaderLines = append(reqHeaderLines, fmt.Sprintf("%s: %s", k, strings.Join(vals, ", ")))
			}
			sort.Strings(reqHeaderLines)
			reqHeaders := strings.Join(reqHeaderLines, "\n")

			// Sort and format response headers (同样打码)
			var respHeaderLines []string
			for k, v := range resp.Header {
				var vals []string
				for _, vv := range v {
					vals = append(vals, maskHeaderValue(k, vv, config.MaskedHeaders))
				}
				respHeaderLines = append(respHeaderLines, fmt.Sprintf("%s: %s", k, strings.Join(vals, ", ")))
			}
			sort.Strings(respHeaderLines)
			respHeaders := strings.Join(respHeaderLines, "\n")

			c.Header("Content-Type", "text/plain; charset=utf-8")
			c.String(resp.StatusCode, fmt.Sprintf("[Request Headers]\n%s\n\n[Response Headers]\n%s\n\n[Body]\n%s", reqHeaders, respHeaders, bodyStr))
			return
		}

		tools.PatchHeader(c, resp.Header)
		c.DataFromReader(
			resp.StatusCode,
			resp.ContentLength,
			resp.Header.Get("Content-Type"),
			resp.Body,
			map[string]string{"X-Service": config.Name},
		)
	}
}

// maskHeaderValue 对命中的敏感 header 值打码。
// 优先按 config.MaskedHeaders 屏蔽; 兜底屏蔽 Authorization、Cookie 以及常见 key/token 头。
func maskHeaderValue(key, value string, masked []string) string {
	for _, m := range masked {
		if strings.EqualFold(key, m) {
			return "***"
		}
	}
	switch strings.ToLower(key) {
	case "authorization", "proxy-authorization", "cookie", "x-api-key", "api-key", "apikey", "x-auth-token", "x-api-token", "x-access-token", "x-session-key":
		return "***"
	}
	return value
}

// stripBodyKeys 从 JSON 对象顶层删除指定键并重新序列化。
// body 不是 JSON 对象(非 JSON / 数组 / 空)时返回 err, 调用方保持原样透传。
// 用 json.RawMessage 保留各值的原始字节(数值/字符串不被重编码走样)。
func stripBodyKeys(body []byte, keys []string) ([]byte, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, err
	}
	drop := make(map[string]bool, len(keys))
	for _, k := range keys {
		drop[k] = true
	}
	for k := range m {
		if drop[k] {
			delete(m, k)
		}
	}
	return json.Marshal(m)
}

// RunProxyRouter starts a gin server that proxies all requests to the provided config
func RunProxyRouter(addr string, config ProxyConfig) {
	if addr == "" {
		return
	}

	r := gin.Default()
	r.Use(middleware.CORSMiddleware())

	// 前置入站 auth 检查(单人自用): 空 token 不装中间件, 保持旧行为。
	if config.InboundToken != "" {
		r.Use(InboundAuthMiddleware(config.InboundToken))
	}

	r.Any("/*any", GenericProxyHandler(config))

	r.Run(addr)
}
