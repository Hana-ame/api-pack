package app

import (
	"net"
	"net/http"
	"os"
	"time"

	_ "github.com/joho/godotenv/autoload"

	"github.com/Hana-ame/api-pack/internal/proxy"
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
func startIfEnv(envKey string, fn func()) {
	if tools.HasEnv(envKey) {
		go fn()
	}
}

// Run 装配并启动 proxy/twimg/pbs 分流服务（原根 main 的职责）。
// 本分支只保留 proxy.moonchan.xyz（root 代理）与 twimg/pbs 分流，
// 其余模块（LLM 反代、ehviewer、chatto、pmsignal、qwen、exhentai 等）全部移除。
func Run() {

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

	// --- twimg/pbs 分流（本分支保留的模块）---
	// 只保留 TwimgProxyV2（pbs/twimg 图片 + video 特殊处理）；
	// 旧版 TwimgProxy / PximgProxy 已随"其他模块"移除。
	startIfEnv("TWIMG_V2", func() { proxy.TwimgProxyV2(os.Getenv("TWIMG_V2")) }) // 127.26.8.10:8080

	// --- proxy.moonchan.xyz（root 代理）---
	// 127.24.11.16:8080 (由 r.Run(os.Getenv("PROXY")) 监听)
	r := gin.Default()

	// 设置 CORS 头
	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.ProxyMiddleware())

	// 注意: 不能用 r.Any("/*any", ...)。gin 的 radix tree 不允许 root 级 catch-all
	// 与任何静态路由共存, 两者顺序都会 panic:
	//   "catch-all wildcard '*any' in new path '/*any' conflicts with existing path segment 'api'"
	// NoRoute 是 gin 官方给的兜底, 不占路由节点, 和静态路由任意顺序都共存。
	// 行为不变: 没匹配到路由的请求同样交给 rootProxyHandler。
	//
	// 目标 host 白名单: 默认不允许任何 host（空 allowlist，拒绝一切），防止本代理被
	// 当成任意目标开放代理刷流量。env PROXY_ALLOW_HOSTS 显式配置后按配置生效（逗号分隔,
	// "*" 表示放开任意 host）。注意: 空/纯分隔符配置解析为 nil 在 Match 里等同不限制,
	// 所以"默认拒绝一切"必须用非 nil 的空 allowlist, 不能依赖 ParseHostAllowlist("")。
	allowSpec := os.Getenv("PROXY_ALLOW_HOSTS")
	if allowSpec == "" {
		r.NoRoute(proxy.RootProxyHandlerAllowHosts(&proxy.HostAllowlist{}))
	} else {
		r.NoRoute(proxy.RootProxyHandlerAllowHosts(proxy.ParseHostAllowlist(allowSpec)))
	}

	r.Run(os.Getenv("PROXY"))
}
