package proxies

import (
	"io"
	"net/http"
	"strings"
	"time"

	middleware "github.com/Hana-ame/api-pack/tools/my_gin_middleware"
	tools "github.com/Hana-ame/api-pack/tools/utils"
	"github.com/gin-gonic/gin"
)

// PassthroughConfig 透传代理的最小配置。
// Timeout 给 0 时不设超时(见 getClient):LLM 上游响应慢, DefaultClient 的 30s
// 会掐断长请求, 所以这里不再依赖它的默认值。
type PassthroughConfig struct {
	Timeout time.Duration
}

// getClient 返回只去掉 Timeout 的 client。
//
// 为什么必须复制: main() 把 http.DefaultClient 换成了带 30s Timeout +
// 自定义 LocalAddr dialer 的 client。这里必须复用它的 Transport
// (连接池、LocalAddr、TLSHandshakeTimeout 都要保留, 否则退化成零值 Transport),
// 但不能再受那 30s 约束 —— LLM 生成可能几十秒。
// 与 generic_proxy.go 里的做法一致(它也是 *http.DefaultClient 再改 Timeout)。
func getClient(timeout time.Duration) *http.Client {
	cc := *http.DefaultClient
	if timeout > 0 {
		cc.Timeout = timeout
	} else {
		cc.Timeout = 0
	}
	return &cc
}

// 与 GenericProxyHandler 的区别(那些都是"除了 CORS 之外"的行为, 这里全部不要):
//   - 不校验/解析 model 字段, 不改请求 body
//   - 不注入 APIKey, 不改 Authorization (前端直接带自己的 key)
//   - 不强制改写 Content-Type
//   - 非 2xx 时不替换成调试回显(原样返回上游的 error body)
//
// 唯一附加的是 CORSMiddleware 注入的 Access-Control-* 响应头。
// 用途: 给只缺 CORS 的第三方 API 补一层同源外壳, 例如 scnet 的 GLM 系列。
// PassthroughProxy 把上游响应原样透传给客户端, 自己不碰任何内容。
func PassthroughProxy(cfg PassthroughConfig, target string) gin.HandlerFunc {
	// 上游是 /v1 版本的 OpenAI 兼容端点, 而且两种拼法都能配:
	//   base = https://api.scnet.cn/api/llm/v1     (/v1 在 base 里)
	//   base = https://api.scnet.cn/api/llm        (/v1 在 path 里)
	// 不管哪种, 都归一成 "base 不含 /v1 + path 带 /v1" 再拼接。否则:
	//   base 带 /v1 时又给 path 补 /v1 → /api/llm/v1/v1/chat/completions, 静默 404。
	// 客户端也两种写法都要能过:
	//   /v1/chat/completions  本项目页面约定 (见 sensenova.html 的 const API)
	//   /chat/completions     通用 OpenAI 客户端习惯
	// 【坑】TrimRight 是删末尾的字符集, 不是删子串:
	//   TrimRight(".../api/llm/v1", "/") == ".../api/llm/v1"  (只去了那个 /)
	// 想去掉末尾的 "/v1" 段要用 HasSuffix + 截断。
	base := strings.TrimRight(target, "/")
	prefix := ""
	if strings.HasSuffix(base, "/v1") {
		// 去掉末尾的 "/v1" 段(含分隔符), 再靠 prefix 补回。
		base = strings.TrimSuffix(base, "/v1")
		prefix = "/v1"
	}

	return func(c *gin.Context) {
		// 读进 buffer: 本进程 http.DefaultClient 带 30s Timeout,
		// 若直接传 body 流, 超时被取消时上游读端会拿到 EOF。
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "read request body: " + err.Error()})
			return
		}

		path := c.Request.URL.Path
		if path != "/" && !strings.HasPrefix(path, "/v1/") && path != "/v1" {
			path = prefix + path
		}

		targetURL := base + path
		req, err := http.NewRequest(c.Request.Method, targetURL, strings.NewReader(string(body)))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "create request: " + err.Error()})
			return
		}

		// 请求头原样转写, 只去掉 Host(由 http.NewRequest 按上游重写)。
		// 注意: 路径归一后, base 带 /v1 时入口 /v1/chat/completions 与
		// /chat/completions 会归到同一条上游地址(.../api/llm/v1/chat/completions)。
		// OpenAI 系 API 没有 /v1/v1/ 这种嵌套路径, 所以这个归并无实际影响。
		for k, vs := range c.Request.Header {
			if k == "Host" {
				continue
			}
			req.Header[k] = vs
		}
		if c.Request.URL.RawQuery != "" {
			req.URL.RawQuery = c.Request.URL.RawQuery
		}

		resp, err := getClient(cfg.Timeout).Do(req)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "upstream request failed: " + err.Error()})
			return
		}
		defer resp.Body.Close()

		tools.PatchHeader(c, resp.Header)
		c.DataFromReader(resp.StatusCode, resp.ContentLength, resp.Header.Get("Content-Type"), resp.Body, nil)
	}
}

// RunPassthroughProxy 起一个只加 CORS 的透传反代。addr 为空时静默跳过
// (.env 没配 = 这个 endpoint 不启动), 与 RunProxyRouter 一致。
func RunPassthroughProxy(addr, target string, timeout time.Duration) {
	if addr == "" {
		return
	}

	r := gin.Default()
	r.Use(middleware.CORSMiddleware())
	r.Any("/*any", PassthroughProxy(PassthroughConfig{Timeout: timeout}, target))

	r.Run(addr)
}
