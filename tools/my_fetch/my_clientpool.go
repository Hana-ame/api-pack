// gin-pack @ 2024-04-06
// azure-go @ 2023-12-21
// eh-web-viewer

package myfetch

import (
	"net/http"
	"net/http/cookiejar"
	"net/url"
)

var (
	jar, _ = cookiejar.New(nil)
)

func SetCookieJar(newJar *cookiejar.Jar) {
	jar = newJar
}

func NewProxyClient(proxyUrl string) *http.Client {
	proxyURL, _ := url.Parse(proxyUrl)
	tr := &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
	}
	return &http.Client{
		Transport: tr,
		Jar:       jar,
	}
}

// client poll
type clientPool struct {
	cnt     uint8
	clients []*http.Client
}

func (cp *clientPool) Client() *http.Client {
	cp.cnt++

	// 1. 先获取原始的 Client 指针
	var original *http.Client
	if len(cp.clients) == 0 {
		original = http.DefaultClient
	} else {
		return cp.clients[int(cp.cnt)%len(cp.clients)]
	}

	// 2. 【关键步骤】浅拷贝 Client 结构体
	// 通过 *original 获取结构体的值，赋值给新变量。
	// 这样 clientCopy 拥有和 original 一样的 Transport（复用连接池）、Timeout 和 Jar，
	// 但修改 clientCopy 不会影响原始的 original 对象。
	clientCopy := *original

	// 3. 修改副本的 CheckRedirect 逻辑
	clientCopy.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		// 返回这个特殊错误，http.Client 会停止跳转并返回当前的 Response (3xx)
		return http.ErrUseLastResponse
	}

	// 4. 返回新结构体的指针
	return &clientCopy
}

func (cp *clientPool) SetClients(clients []*http.Client) {
	cp.clients = clients
}

func NewClientPool(clients []*http.Client) *clientPool {
	return &clientPool{
		cnt:     0,
		clients: clients,
	}
}

// public methods

var DefaultClientPool *clientPool

func Client() *http.Client {
	return DefaultClientPool.Client()
}

func SetClients(clients []*http.Client) {
	DefaultClientPool.SetClients(clients)
}
