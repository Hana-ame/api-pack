package exhentai

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	myfetch "github.com/Hana-ame/api-pack/tools/my_fetch/v2"
	"github.com/joho/godotenv"
	tls "github.com/refraction-networking/utls"
)

var defaultClient = &http.Client{
	Transport: &http.Transport{
		DialContext: (&net.Dialer{
			LocalAddr: &net.TCPAddr{
				IP:   nil,
				Port: 0,
			},
			Timeout:   3 * time.Second,
			KeepAlive: 3 * time.Second,
		}).DialContext,
		MaxIdleConns:        100,
		IdleConnTimeout:     10 * time.Second,
		TLSHandshakeTimeout: 3 * time.Second,
	},
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

// 定义上下文键
type contextKey string

const fingerprintKey contextKey = "tls_fingerprint"

// client 保持不变
type client struct {
	*myfetch.Client
	*netip.Addr
}

const (
	// DefaultRotationThreshold 单个槽位的轮换阈值
	DefaultRotationThreshold = 1000
	// DefaultCleanupDelay 删除旧 IP 的延迟时间
	DefaultCleanupDelay = 6 * time.Second
	// DefaultPoolSize 默认并发 IP 数量
	DefaultPoolSize = 5
)

// -------------------------------------------------------
// utlsDialer: 使用 uTLS 的拨号器
// -------------------------------------------------------
type utlsDialer struct {
	targetAddr  string            // Cloudflare IP
	localIP     net.IP            // 本地生成的 IP
	netDialer   *net.Dialer       // 基础拨号器
	fingerprint tls.ClientHelloID // TLS 指纹
}

// DialTLSContext 使用 uTLS 建立 TLS 连接
func (d *utlsDialer) DialTLSContext(ctx context.Context, network, addr string) (net.Conn, error) {
	// 从上下文中获取指纹（如果存在）
	id, ok := ctx.Value(fingerprintKey).(tls.ClientHelloID)
	if !ok {
		// 使用拨号器自身的指纹作为默认值
		id = d.fingerprint
	}

	// 1. 建立 TCP 连接
	d.netDialer.LocalAddr = &net.TCPAddr{IP: d.localIP}
	tcpConn, err := d.netDialer.DialContext(ctx, "tcp6", d.targetAddr)
	if err != nil {
		return nil, fmt.Errorf("TCP dial failed: %w", err)
	}

	// 2. 使用 uTLS 包装连接
	config := &tls.Config{
		ServerName: "exhentai.org", // SNI 必须是 exhentai.org
	}

	uConn := tls.UClient(tcpConn, config, id)

	// 3. 执行 TLS 握手
	if err := uConn.HandshakeContext(ctx); err != nil {
		tcpConn.Close()
		return nil, fmt.Errorf("uTLS handshake failed: %w", err)
	}

	return uConn, nil
}

// -------------------------------------------------------
// ClientSlot: 代表连接池中的一个"槽位"，负责管理单个IP的生命周期
// -------------------------------------------------------
type clientSlot struct {
	id        int              // 槽位ID，用于日志区分
	ipManager *myfetch.Manager // 用于生成/删除IP

	// currentClientHolder 原子存储当前的 *client
	currentClientHolder atomic.Value

	// usageCounter 当前 IP 的使用次数
	usageCounter int64

	// rotationMu 保护轮换逻辑
	rotationMu sync.Mutex
	// nextClient 预备好的下一个客户端
	nextClient *client

	// targetAddr 目标地址 (Cloudflare IP)
	targetAddr string
	// defaultFingerprint 默认的 TLS 指纹
	defaultFingerprint tls.ClientHelloID
}

// newClientSlot 初始化一个槽位
func newClientSlot(id int, manager *myfetch.Manager, targetAddr string, fingerprint tls.ClientHelloID) (*clientSlot, error) {
	slot := &clientSlot{
		id:                 id,
		ipManager:          manager,
		targetAddr:         targetAddr,
		defaultFingerprint: fingerprint,
	}

	// 1. 初始化当前 client
	current, err := slot.prepareNewClient()
	if err != nil {
		return nil, err
	}
	slot.currentClientHolder.Store(current)

	// 2. 提前准备下一个 client
	next, err := slot.prepareNewClient()
	if err != nil {
		_ = slot.ipManager.DelAddr(*(current.Addr))
		return nil, err
	}
	slot.nextClient = next

	return slot, nil
}

// prepareNewClient 生成 IP 并创建使用 uTLS 的 Client
func (s *clientSlot) prepareNewClient() (*client, error) {
	ip, err := s.ipManager.GenerateIP()
	if err != nil {
		return nil, fmt.Errorf("slot %d generate ip failed: %w", s.id, err)
	}

	if err := s.ipManager.AddAddr(ip); err != nil {
		return nil, fmt.Errorf("slot %d add addr failed: %w", s.id, err)
	}

	// 创建使用 uTLS 的 Transport
	utlsDialer := &utlsDialer{
		targetAddr: s.targetAddr,
		localIP:    ip.AsSlice(),
		netDialer: &net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		},
		fingerprint: s.defaultFingerprint,
	}

	c := &client{
		Addr: &ip,
		Client: &myfetch.Client{
			Client: &http.Client{
				Transport: &http.Transport{
					// 使用自定义的 DialTLSContext
					DialTLSContext: utlsDialer.DialTLSContext,
					// 注意：这里不再使用 DialContext，因为 TLS 连接由 DialTLSContext 处理
					// 但是我们需要设置 TCP 拨号器用于非 TLS 连接（虽然这里可能不需要）
					DialContext: (&net.Dialer{
						LocalAddr: &net.TCPAddr{IP: ip.AsSlice()},
						Timeout:   5 * time.Second,
						KeepAlive: 30 * time.Second,
					}).DialContext,
					MaxIdleConns:        100,
					IdleConnTimeout:     10 * time.Second,
					TLSHandshakeTimeout: 10 * time.Second, // 增加超时时间
				},
				CheckRedirect: func(req *http.Request, via []*http.Request) error {
					return http.ErrUseLastResponse
				},
			},
		},
	}
	return c, nil
}

// getCurrentClient 获取当前客户端
func (s *clientSlot) getCurrentClient() *client {
	return s.currentClientHolder.Load().(*client)
}

// getFingerprintFromUA 根据 User-Agent 获取对应的 TLS 指纹
func getFingerprintFromUA(ua string) tls.ClientHelloID {
	ua = strings.ToLower(ua)
	if strings.Contains(ua, "firefox") {
		return tls.HelloFirefox_Auto
	}
	if strings.Contains(ua, "iphone") || strings.Contains(ua, "ipad") {
		return tls.HelloIOS_Auto
	}
	if strings.Contains(ua, "android") {
		return tls.HelloAndroid_11_OkHttp // 移动应用常用
	}
	// 默认使用 Chrome (Desktop Chrome, Edge, Safari)
	return tls.HelloChrome_Auto
}

// DoProxyRequest 执行代理请求，使用用户的真实 User-Agent 确定 TLS 指纹
func (s *clientSlot) DoProxyRequest(userReq *http.Request) (*http.Response, error) {
	// 1. 根据真实用户的 User-Agent 确定指纹
	ua := userReq.Header.Get("User-Agent")
	fingerprint := getFingerprintFromUA(ua)

	// 2. 将指纹放入上下文
	ctx := context.WithValue(userReq.Context(), fingerprintKey, fingerprint)

	// 3. 创建到 ExHentai 的请求
	proxyReq, _ := http.NewRequestWithContext(ctx, userReq.Method, userReq.URL.String(), userReq.Body)

	// 4. 复制所有用户头（包括 UA 和 Cookies）
	for k, vv := range userReq.Header {
		for _, v := range vv {
			proxyReq.Header.Add(k, v)
		}
	}

	// 5. 使用当前客户端执行请求
	current := s.getCurrentClient()
	return current.Fetch(proxyReq.Method, proxyReq.URL.String(), proxyReq.Header, proxyReq.Body)
}

// execute 执行请求并处理该槽位的轮换逻辑（兼容旧版）
func (s *clientSlot) execute(method, url string, header http.Header, body io.Reader) (*http.Response, error) {
	// 1. 获取当前客户端
	current := s.getCurrentClient()

	// 2. 增加该槽位的使用计数
	count := atomic.AddInt64(&s.usageCounter, 1)

	// 3. 检查是否需要轮换 (阈值检查)
	if count >= DefaultRotationThreshold {
		// 尝试获取锁进行轮换 (TryLock 非阻塞)
		if s.rotationMu.TryLock() {
			// 只有获取到锁的请求才执行轮换，其他请求继续用旧的
			s.performRotation(current)
			// 轮换完尝试拿新的（虽然大部分情况可能拿不到最新的，但不影响）
			current = s.getCurrentClient()
		}
	}

	// 4. 检查是否有 User-Agent 头，决定是否使用指纹
	var ctx context.Context
	if ua := header.Get("User-Agent"); ua != "" {
		fingerprint := getFingerprintFromUA(ua)
		ctx = context.WithValue(context.Background(), fingerprintKey, fingerprint)
	} else {
		ctx = context.Background()
	}

	// 5. 创建带有上下文的请求
	req, _ := http.NewRequestWithContext(ctx, method, url, body)
	req.Header = header

	// 6. 发起请求
	return current.Fetch(req.Method, req.URL.String(), req.Header, req.Body)
}

// performRotation 执行客户端轮换
func (s *clientSlot) performRotation(oldClient *client) {
	// 检查 next 是否就绪
	if s.nextClient == nil {
		s.rotationMu.Unlock()
		atomic.AddInt64(&s.usageCounter, -100) // 临时回退计数器，稍后重试
		return
	}

	next := s.nextClient

	// 切换
	s.currentClientHolder.Store(next)
	s.nextClient = nil
	atomic.StoreInt64(&s.usageCounter, 0) // 重置计数
	s.rotationMu.Unlock()

	log.Printf("[Slot %d] Rotated: %s -> %s", s.id, oldClient.Addr, next.Addr)

	// 后台准备
	go s.backgroundPrepare(oldClient)
}

func (s *clientSlot) backgroundPrepare(oldClient *client) {
	// 准备新 IP
	newNext, err := s.prepareNewClient()
	if err != nil {
		log.Printf("[Slot %d] ERROR preparing next: %v", s.id, err)
	} else {
		s.rotationMu.Lock()
		s.nextClient = newNext
		s.rotationMu.Unlock()
		log.Printf("[Slot %d] Backup ready: %s", s.id, newNext.Addr)
	}

	// 延迟清理旧 IP
	time.Sleep(DefaultCleanupDelay)
	oldClient.Client.CloseIdleConnections()
	if err := s.ipManager.DelAddr(*(oldClient.Addr)); err != nil {
		log.Printf("[Slot %d] cleanup error: %v", s.id, err)
	}
}

// -------------------------------------------------------
// IPRotator: 管理器，负责负载均衡
// -------------------------------------------------------

type IPRotator struct {
	slots     []*clientSlot // 固定数量的槽位池
	rrCounter uint64        // Round-Robin 计数器
}

// NewIPRotator 创建包含多个 IP 的 Rotator
// poolSize: 同时维护多少个 IP (例如 5 或 10)
// targetAddr: Cloudflare IP 地址 (例如 "104.20.134.65:443")
// fingerprint: 默认的 TLS 指纹
func NewIPRotator(manager *myfetch.Manager, poolSize int, targetAddr string, fingerprint tls.ClientHelloID) (*IPRotator, error) {
	if poolSize <= 0 {
		poolSize = DefaultPoolSize
	}

	if targetAddr == "" {
		targetAddr = "exhentai.org:443" // 默认目标地址
	}

	rotator := &IPRotator{
		slots: make([]*clientSlot, poolSize),
	}

	log.Printf("Initializing IPRotator with pool size: %d, target: %s...", poolSize, targetAddr)

	// 并行初始化所有槽位，加快启动速度
	var wg sync.WaitGroup
	var initErr error
	var errMu sync.Mutex

	for i := 0; i < poolSize; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			slot, err := newClientSlot(idx, manager, targetAddr, fingerprint)
			if err != nil {
				errMu.Lock()
				initErr = err
				errMu.Unlock()
				return
			}
			rotator.slots[idx] = slot
		}(i)
	}
	wg.Wait()

	if initErr != nil {
		// 如果初始化失败，这里应该做清理逻辑，把已经创建好的都销毁
		// 为简化代码，这里直接返回错误
		return nil, fmt.Errorf("failed to init slots: %w", initErr)
	}

	log.Println("IPRotator initialized successfully.")
	return rotator, nil
}

// Fetch 实现了负载均衡的请求分发
func (r *IPRotator) Fetch(method, url string, header http.Header, body io.Reader) (*http.Response, error) {
	// 1. Round-Robin 算法选择一个槽位
	// 使用原子操作递增全局计数器
	reqNum := atomic.AddUint64(&r.rrCounter, 1)

	// 取模得到索引。注意 len(r.slots) 是常量（初始化后不变），所以安全
	slotIdx := reqNum % uint64(len(r.slots))

	selectedSlot := r.slots[slotIdx]

	// 2. 委托给选中的槽位执行
	return selectedSlot.execute(method, url, header, body)
}

// DoProxyRequest 代理请求，自动使用用户 User-Agent 的 TLS 指纹
func (r *IPRotator) DoProxyRequest(userReq *http.Request) (*http.Response, error) {
	// 1. Round-Robin 算法选择一个槽位
	reqNum := atomic.AddUint64(&r.rrCounter, 1)
	slotIdx := reqNum % uint64(len(r.slots))
	selectedSlot := r.slots[slotIdx]

	// 2. 委托给选中的槽位执行代理请求
	return selectedSlot.DoProxyRequest(userReq)
}

// Run 模拟运行
func Run(addr string) {
	if addr == "" {
		return
	}
	godotenv.Load(".env")

	manager, err := myfetch.NewManager("sit1", "")
	if err != nil {
		log.Fatalf("Failed to create manager: %v", err)
	}

	// 使用 Cloudflare IP 地址和默认的 Chrome 指纹
	rotator, err := NewIPRotator(manager, 10, "exhentai.org:443", tls.HelloChrome_Auto)
	if err != nil {
		log.Fatalf("Failed to create IP rotator: %v", err)
	}

	ExhProxy(rotator, addr)
}
