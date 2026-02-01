package exhentai

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/netip"
	"os"
	"sync"
	"sync/atomic"
	"time"

	myfetch "github.com/Hana-ame/api-pack/tools/my_fetch/v2"
	tools "github.com/Hana-ame/api-pack/tools/utils"
	"github.com/joho/godotenv"
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
// ClientSlot: 代表连接池中的一个“槽位”，负责管理单个IP的生命周期
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
}

// newClientSlot 初始化一个槽位
func newClientSlot(id int, manager *myfetch.Manager) (*clientSlot, error) {
	slot := &clientSlot{
		id:        id,
		ipManager: manager,
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

// 备用逻辑

type dialer struct {
	address string
	*net.Dialer
}

func (dialer dialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return dialer.Dialer.DialContext(ctx, network, dialer.address)
}
func (s *clientSlot) prepareNewClient() (*client, error) {
	ip, err := s.ipManager.GenerateIP()
	if err != nil {
		return nil, fmt.Errorf("slot %d generate ip failed: %w", s.id, err)
	}

	if err := s.ipManager.AddAddr(ip); err != nil {
		return nil, fmt.Errorf("slot %d add addr failed: %w", s.id, err)
	}

	// 1. Specify the exact Cloudflare IP you want to hit (like /etc/hosts)
	// You can rotate these or hardcode one: [2a06:98c1:3120::]:443 or [2a06:98c1:3121::]:443
	targetIP := tools.Or(os.Getenv("EX_TARGET_IP"), "exhentai.org:443")

	c := &client{
		Addr: &ip,
		Client: &myfetch.Client{
			Client: &http.Client{
				Transport: &http.Transport{
					// Use the IP address for the TCP connection
					DialContext: (dialer{targetIP, &net.Dialer{
						LocalAddr: &net.TCPAddr{
							IP: ip.AsSlice(),
						},
						Timeout:   5 * time.Second,
						KeepAlive: 30 * time.Second,
					}}).DialContext,

					// 2. CRITICAL: You must tell TLS that we are still talking to exhentai.org
					// Otherwise, the SNI will be the IP address and the handshake will fail.
					TLSClientConfig: &tls.Config{
						ServerName: "exhentai.org",
					},

					MaxIdleConns:        100,
					IdleConnTimeout:     10 * time.Second,
					TLSHandshakeTimeout: 5 * time.Second,
				},
				CheckRedirect: func(req *http.Request, via []*http.Request) error {
					return http.ErrUseLastResponse
				},
			},
		},
	}
	return c, nil
}

// 正常逻辑
/*

// prepareNewClient 生成 IP 并创建 Client
func (s *clientSlot) prepareNewClient() (*client, error) {
	ip, err := s.ipManager.GenerateIP()
	if err != nil {
		return nil, fmt.Errorf("slot %d generate ip failed: %w", s.id, err)
	}

	if err := s.ipManager.AddAddr(ip); err != nil {
		return nil, fmt.Errorf("slot %d add addr failed: %w", s.id, err)
	}

	c := &client{
		Addr: &ip,
		Client: &myfetch.Client{
			Client: &http.Client{
				Transport: &http.Transport{
					DialContext: (&net.Dialer{
						LocalAddr: &net.TCPAddr{IP: ip.AsSlice()},
						Timeout:   3 * time.Second,
						KeepAlive: 30 * time.Second,
					}).DialContext,
					MaxIdleConns:        100,
					IdleConnTimeout:     10 * time.Second,
					TLSHandshakeTimeout: 3 * time.Second,
				},
				CheckRedirect: func(req *http.Request, via []*http.Request) error {
					return http.ErrUseLastResponse
				},
			},
		},
	}
	return c, nil
}

*/
func (s *clientSlot) getCurrentClient() *client {
	return s.currentClientHolder.Load().(*client)
}

// execute 执行请求并处理该槽位的轮换逻辑
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

	// 4. 发起请求
	return current.Fetch(method, url, header, body)
}

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
func NewIPRotator(manager *myfetch.Manager, poolSize int) (*IPRotator, error) {
	if poolSize <= 0 {
		poolSize = DefaultPoolSize
	}

	rotator := &IPRotator{
		slots: make([]*clientSlot, poolSize),
	}

	log.Printf("Initializing IPRotator with pool size: %d...", poolSize)

	// 并行初始化所有槽位，加快启动速度
	var wg sync.WaitGroup
	var initErr error
	var errMu sync.Mutex

	for i := 0; i < poolSize; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			slot, err := newClientSlot(idx, manager)
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

	// 比如开启 10 个并发 IP
	rotator, err := NewIPRotator(manager, 10)
	if err != nil {
		log.Fatalf("Failed to create IP rotator: %v", err)
	}

	ExhProxy(rotator, addr)
}
