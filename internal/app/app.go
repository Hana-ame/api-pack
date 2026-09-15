package app

import (
	"net"
	"net/http"
	"os"
	"time"

	_ "github.com/joho/godotenv/autoload"

	"github.com/Hana-ame/api-pack/internal/exhentai"
	"github.com/Hana-ame/api-pack/internal/exhentai/modify"
	"github.com/Hana-ame/api-pack/internal/exhentai/stream"
	"github.com/Hana-ame/api-pack/internal/proxy"
	"github.com/Hana-ame/api-pack/internal/service/bilibili"
	"github.com/Hana-ame/api-pack/internal/service/chatto"
	"github.com/Hana-ame/api-pack/internal/service/moonchan"
	"github.com/Hana-ame/api-pack/internal/service/qwen"
	"github.com/Hana-ame/api-pack/pkg/debug"
	middleware "github.com/Hana-ame/api-pack/pkg/ginutil/middleware"
	tools "github.com/Hana-ame/api-pack/pkg/utils"
	"github.com/gin-gonic/gin"
)

func localTCPAddrFromEnv() *net.TCPAddr {
	if ipStr := os.Getenv("LOCAL_IP"); ipStr != "" {
		if ip := net.ParseIP(ipStr); ip != nil {
			return &net.TCPAddr{IP: ip}
		}
	}
	// 返回 nil 默认让系统选择（解决 127.0.0.2 无法访问公网的问题）
	return nil
}

// startIfEnv 是所有「按 env 门控启动服务」的唯一入口：
// env 非空则异步运行 fn（fn 内部自行读取实际监听地址）。
// 重构前这里混用三种写法：os.Getenv(x)!=""、tools.HasEnv(x)、以及不开门控
// 直接 go X.Run(os.Getenv(x)) 靠 Run 内部空地址兜底；统一成此函数。
func startIfEnv(envKey string, fn func()) {
	if tools.HasEnv(envKey) {
		go fn()
	}
}

// Run 装配并启动全部服务(原根 main 的职责)。
func Run() {

	// 我不行了。
	http.DefaultClient = &http.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				LocalAddr: localTCPAddrFromEnv(),
				Timeout:   15 * time.Second,
				KeepAlive: 90 * time.Second,
			}).DialContext,
			MaxIdleConns:        256,
			IdleConnTimeout:     10 * time.Second,
			TLSHandshakeTimeout: 30 * time.Second,
		},
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // 保持原样，将 301/302 转发给客户端
		},
	}

	debug.LogLevel = debug.Trace

	// --- 固定监听地址、env 仅作开关 ---
	startIfEnv("NYAA_PROXY", proxy.NyaaProxy)       // 127.25.23.4:8080
	startIfEnv("SUKEBEI_PROXY", proxy.SukebeiProxy) // 127.25.23.5:8080

	// --- OpenAI 兼容 / CORS 反代路由 ---
	startIfEnv("GROQ_PROXY", func() {
		proxy.RunProxyRouter(os.Getenv("GROQ_PROXY"), proxy.ProxyConfig{
			Name:          "groq",
			Endpoint:      "https://api.groq.com",
			APIKey:        os.Getenv("GROQ_API_KEY"),
			FreeModelsAll: true,
			MaskedHeaders: []string{
				"Authorization",
				"X-Api-Key",
				"Cookie",
			},
		})
	})
	startIfEnv("SILICONFLOW_PROXY", func() {
		proxy.RunProxyRouter(os.Getenv("SILICONFLOW_PROXY"), proxy.ProxyConfig{
			Name:     "siliconflow",
			Endpoint: "https://api.siliconflow.cn",
			APIKey:   os.Getenv("SILICONFLOW_API_KEY"),
			FreeModels: map[string]bool{
				"Qwen/Qwen3.5-4B":                         true,
				"PaddlePaddle/PaddleOCR-VL-1.5":           true,
				"deepseek-ai/DeepSeek-R1-Distill-Qwen-7B": true,
				"THUDM/GLM-4.1V-9B-Thinking":              true,
				"PaddlePaddle/PaddleOCR-VL":               true,
				"deepseek-ai/DeepSeek-OCR":                true,
				"Qwen/Qwen3-8B":                           true,
				"tencent/Hunyuan-MT-7B":                   true,
				"deepseek-ai/DeepSeek-R1-0528-Qwen3-8B":   true,
				"THUDM/GLM-Z1-9B-0414":                    true,
				"Qwen/Qwen2.5-7B-Instruct":                true,
				"THUDM/GLM-4-9B-0414":                     true,
				"internlm/internlm2_5-7b-chat":            true,
			},
			MaskedHeaders: []string{
				"Authorization",
				"X-Api-Key",
				"Cookie",
			},
		})
	})
	startIfEnv("GEMINI_PROXY", func() {
		proxy.RunProxyRouter(os.Getenv("GEMINI_PROXY"), proxy.ProxyConfig{
			Name:          "gemini",
			Endpoint:      "https://generativelanguage.googleapis.com",
			FreeModelsAll: true,
			MaskedHeaders: []string{
				"Authorization",
				"X-Api-Key",
				"Cookie",
			},
		})
	})
	// SenseNova 生图 API (token.sensenova.cn) CORS 转发:
	// 只对 sensenova-u1.5-lite/u1-fast 两个模型在客户端未传 key 时注入 APIKey,
	// 其他情况透传客户端的 Authorization (FreeModels 模式)。
	startIfEnv("SENSENOVA_PROXY", func() {
		proxy.RunProxyRouter(os.Getenv("SENSENOVA_PROXY"), proxy.ProxyConfig{
			Name:     "sensenova",
			Endpoint: "https://token.sensenova.cn",
			APIKey:   os.Getenv("SENSENOVA_API_KEY"),
			FreeModels: map[string]bool{
				"sensenova-u1.5-lite":      true,
				"sensenova-u1-fast":        true,
				"sensenova-6.8-flash-lite": true,
			},
			Timeout: 180 * time.Second,
			MaskedHeaders: []string{
				"Authorization",
				"X-Api-Key",
				"Cookie",
			},
		})
	})
	// SCNet (api.scnet.cn) GLM 系列 OpenAI 兼容端点:
	// 纯 CORS 透传反代(不注入 key,客户端自带 Bearer)。
	// Endpoint 不含 /v1,客户端 /api/llm/v1/chat/completions 原样透传为
	// https://api.scnet.cn/api/llm/v1/chat/completions。
	startIfEnv("SCNET_PROXY", func() {
		proxy.RunProxyRouter(os.Getenv("SCNET_PROXY"), proxy.ProxyConfig{
			Name:     "scnet",
			Endpoint: "https://api.scnet.cn",
			Timeout:  180 * time.Second,
			MaskedHeaders: []string{
				"Authorization",
				"X-Api-Key",
				"Cookie",
			},
		})
	})
	// Xingyue (api.xingyueapi.com) OpenAI 兼容端点 CORS 反代:
	// Endpoint 不含 /v1, 客户端 /v1/chat/completions 原样透传为
	// https://api.xingyueapi.com/v1/chat/completions。
	// env:
	//   XINGYUE_PROXY            监听地址(启动开关)
	//   XINGYUE_INBOUND_TOKEN    前置入站 auth 检查(单人自用)的正确入站 token;
	//                            空 = fail-closed, xingyue 代理不启动
	//   XINGYUE_MODEL_TOKENS     出站 model token 组, spec 格式 "<source>[#<jsonpath>]":
	//                              /root/.secrets/xingyue-tokens.json#$.xingyue.tokens
	//                              env://XINGYUE_MODEL_TOKENS_JSON#$.xingyue.tokens
	//                            命中模型的请求上游 Authorization 被强制改写(组内轮转),
	//                            文件形态 mtime/大小变化 5s 内热重载, 轮换 token 不重启。
	startIfEnv("XINGYUE_PROXY", func() {
		var tokens *proxy.ModelTokenTable
		if spec := os.Getenv("XINGYUE_MODEL_TOKENS"); spec != "" {
			t, err := proxy.NewModelTokenTable(spec)
			if err != nil {
				// 宁可不启动也不带错 key 上线: 只跳过本代理, 不影响进程内其他服务
				debug.E("xingyue", "模型 token 表加载失败, xingyue 代理不启动: "+err.Error())
				return
			}
			tokens = t
			debug.I("xingyue", "模型 token 表就绪: "+t.Describe())
		}
		if os.Getenv("XINGYUE_INBOUND_TOKEN") == "" {
			// fail-closed: 设了 XINGYUE_PROXY 但没给入站 token, 宁可不启动也不裸奔
			debug.E("xingyue", "XINGYUE_INBOUND_TOKEN 未设置: xingyue 代理不启动(fail-closed)")
			return
		}
		proxy.RunProxyRouter(os.Getenv("XINGYUE_PROXY"), proxy.ProxyConfig{
			Name:         "xingyue",
			Endpoint:     "https://api.xingyueapi.com",
			Timeout:      180 * time.Second,
			InboundToken: os.Getenv("XINGYUE_INBOUND_TOKEN"),
			ModelTokens:  tokens,
			MaskedHeaders: []string{
				"Authorization",
				"X-Api-Key",
				"Cookie",
			},
		})
	})
	// Maomaoai (api.maomaoai.pro, New API 网关) OpenAI 兼容端点 CORS 反代:
	// 与 xingyue 同款——单人入站门禁 + 按模型注入不同出站 token + token 表热重载。
	// Endpoint 不含 /v1, 客户端 /v1/chat/completions 原样透传为
	// https://api.maomaoai.pro/v1/chat/completions。
	// env:
	//   MAOMAOAI_PROXY            监听地址(启动开关)
	//   MAOMAOAI_INBOUND_TOKEN    前置入站 auth 检查(单人自用)的正确入站 token;
	//                             空 = fail-closed, maomaoai 代理不启动
	//   MAOMAOAI_MODEL_TOKENS     出站 model token 组, spec 格式 "<source>[#<jsonpath>]":
	//                               /root/.secrets/maomaoai-tokens.json#$.maomaoai.tokens
	//                               env://MAOMAOAI_MODEL_TOKENS_JSON#$.maomaoai.tokens
	//                             命中模型的请求上游 Authorization 被强制改写(组内轮转),
	//                             文件形态 mtime/大小变化 5s 内热重载, 轮换 token 不重启。
	startIfEnv("MAOMAOAI_PROXY", func() {
		var tokens *proxy.ModelTokenTable
		if spec := os.Getenv("MAOMAOAI_MODEL_TOKENS"); spec != "" {
			t, err := proxy.NewModelTokenTable(spec)
			if err != nil {
				// 宁可不启动也不带错 key 上线: 只跳过本代理, 不影响进程内其他服务
				debug.E("maomaoai", "模型 token 表加载失败, maomaoai 代理不启动: "+err.Error())
				return
			}
			tokens = t
			debug.I("maomaoai", "模型 token 表就绪: "+t.Describe())
		}
		if os.Getenv("MAOMAOAI_INBOUND_TOKEN") == "" {
			// fail-closed: 设了 MAOMAOAI_PROXY 但没给入站 token, 宁可不启动也不裸奔
			debug.E("maomaoai", "MAOMAOAI_INBOUND_TOKEN 未设置: maomaoai 代理不启动(fail-closed)")
			return
		}
		proxy.RunProxyRouter(os.Getenv("MAOMAOAI_PROXY"), proxy.ProxyConfig{
			Name:         "maomaoai",
			Endpoint:     "https://api.maomaoai.pro",
			Timeout:      180 * time.Second,
			InboundToken: os.Getenv("MAOMAOAI_INBOUND_TOKEN"),
			ModelTokens:  tokens,
			MaskedHeaders: []string{
				"Authorization",
				"X-Api-Key",
				"Cookie",
			},
		})
	})

	// --- 各业务服务，env 值即监听地址 ---
	startIfEnv("SHIJIMA", func() { moonchan.Run(os.Getenv("SHIJIMA")) })     // 127.25.5.18:8080
	startIfEnv("CHATTO_REG", func() { chatto.Run(os.Getenv("CHATTO_REG")) }) // 127.26.8.2:8080

	// go EhProxy() //127.25.23.6:8080
	// go pastejson.Run(os.Getenv("PASTEJSON"), os.Getenv("PASTEJSON_CONN_STR")) // 127.25.9.10:8080

	startIfEnv("TWIMG", func() { proxy.TwimgProxy(os.Getenv("TWIMG")) })         // 127.25.9.15:8080
	startIfEnv("TWIMG_V2", func() { proxy.TwimgProxyV2(os.Getenv("TWIMG_V2")) }) // 127.26.8.10:8080
	startIfEnv("PXIMG", func() { proxy.PximgProxy(os.Getenv("PXIMG")) })         // 127.25.9.16:8080

	startIfEnv("ECHO_JSON", proxy.EchoJSON) // 127.25.23.101:8080

	startIfEnv("QWEN_PROXY", func() { qwen.Run(os.Getenv("QWEN_PROXY")) }) // 127.25.12.16:8080

	startIfEnv("EX_PROXY", func() { exhentai.Run(os.Getenv("EX_PROXY")) })
	startIfEnv("EX_STREAM", func() { stream.Run(os.Getenv("EX_STREAM")) })
	startIfEnv("EX_MODIFY", func() { modify.Run(os.Getenv("EX_MODIFY")) }) // env: EXHENTAI_ENDPOINT

	// proxy.moonchan.xyz
	// 127.24.11.16:8080 (由 r.Run(os.Getenv("PROXY")) 监听)
	// 创建 Gin 引擎
	r := gin.Default()

	// 设置 CORS 头
	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.ProxyMiddleware())

	// 2. 路由处理函数
	bilibili.Register(r) // /api/v2/bili 封面 + 页面

	// 注意: 不能用 r.Any("/*any", ...)。gin 的 radix tree 不允许 root 级 catch-all
	// 与任何静态路由共存, 两者顺序都会 panic:
	//   "catch-all wildcard '*any' in new path '/*any' conflicts with existing path segment 'api'"
	// NoRoute 是 gin 官方给的兜底, 不占路由节点, 和静态路由任意顺序都共存。
	// 行为不变: 没匹配到路由的请求同样交给 rootProxyHandler。
	r.NoRoute(proxy.RootProxyHandler)

	r.Run(os.Getenv("PROXY"))
}
