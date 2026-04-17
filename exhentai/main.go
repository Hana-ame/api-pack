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
	"strconv"
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
			KeepAlive: 10 * time.Second,
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
	DefaultCleanupDelay = 30 * time.Second
	// DefaultPoolSize 默认并发 IP 数量
	DefaultPoolSize = 3
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

// dialer 用于强制指定目标IP
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

	targetIP := tools.Or(os.Getenv("EX_TARGET_IP"), "exhentai.org:443")

	c := &client{
		Addr: &ip,
		Client: &myfetch.Client{
			Client: &http.Client{
				Timeout: 10 * time.Second,
				Transport: &http.Transport{
					DialContext: (dialer{targetIP, &net.Dialer{
						LocalAddr: &net.TCPAddr{
							IP: ip.AsSlice(),
						},
						Timeout:   5 * time.Second,
						KeepAlive: 10 * time.Second,
					}}).DialContext,

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

func (s *clientSlot) getCurrentClient() *client {
	return s.currentClientHolder.Load().(*client)
}

// execute 执行请求并处理该槽位的轮换逻辑
// 修改点：增加了对 502 等 5xx 错误的处理，强制触发轮换
func (s *clientSlot) execute(method, url string, header http.Header, body io.Reader) (*http.Response, error) {
	// 1. 获取当前客户端
	current := s.getCurrentClient()

	// 2. 增加该槽位的使用计数
	count := atomic.AddInt64(&s.usageCounter, 1)

	// 3. 检查是否需要轮换 (阈值检查)
	if count >= DefaultRotationThreshold {
		// 尝试获取锁进行轮换 (TryLock 非阻塞)
		if s.rotationMu.TryLock() {
			s.performRotation(current)
			current = s.getCurrentClient()
		}
	}

	// 4. 发起请求
	resp, err := current.Fetch(method, url, header, body)

	// --- 修改逻辑开始 ---
	// 情况 A: 网络层错误
	if err != nil {
		atomic.AddInt64(&s.usageCounter, 50) // 加速轮换
		return resp, err
	}

	// 情况 B: HTTP 服务端错误 (502 Bad Gateway, 503 Service Unavailable 等)
	// 原因：这通常意味着该 IP 的连接已被服务端重置或限流。
	// 解决：必须强制轮换该 IP，丢弃旧的连接池。
	if resp.StatusCode >= 500 {
		log.Printf("[Slot %d] Server returned %d. Forcing connection pool reset for IP %s", s.id, resp.StatusCode, current.Addr)

		// 强制触发轮换：将计数器直接推过阈值
		atomic.StoreInt64(&s.usageCounter, DefaultRotationThreshold)

		// 尝试立即执行轮换
		if s.rotationMu.TryLock() {
			s.performRotation(current)
		}
	}
	// --- 修改逻辑结束 ---

	resp.Header.Add("X-SERVED-BY", current.Addr.String())
	return resp, err
}

func (s *clientSlot) performRotation(oldClient *client) {
	// 检查 next 是否就绪
	if s.nextClient == nil {
		// --- 修改点：如果 nextClient 没准备好，同步阻塞生成，防止无 IP 可用 ---
		log.Printf("[Slot %d] Next client not ready, generating synchronously...", s.id)
		newNext, err := s.prepareNewClient()
		if err != nil {
			log.Printf("[Slot %d] CRITICAL: Failed to generate new client: %v", s.id, err)
			s.rotationMu.Unlock()
			// 保持计数器高位，下次继续尝试
			return
		}
		s.nextClient = newNext
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

	backupClient *myfetch.Client
}

// NewIPRotator 创建包含多个 IP 的 Rotator
func NewIPRotator(manager *myfetch.Manager, poolSize int, backupIP string) (*IPRotator, error) {
	if poolSize <= 0 {
		poolSize = DefaultPoolSize
	}

	rotator := &IPRotator{
		slots: make([]*clientSlot, poolSize),
	}

	log.Printf("Initializing IPRotator with pool size: %d...", poolSize)

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
		return nil, fmt.Errorf("failed to init slots: %w", initErr)
	}

	if backupIP != "" {
		if addr, err := netip.ParseAddr(backupIP); err == nil {
			rotator.backupClient = &myfetch.Client{
				Client: &http.Client{
					Transport: &http.Transport{
						DialContext: (&net.Dialer{
							LocalAddr: &net.TCPAddr{
								IP: addr.AsSlice(),
							},
							Timeout:   5 * time.Second,
							KeepAlive: 10 * time.Second,
						}).DialContext,
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
			}
			log.Println("IPRotator backup IP ready", rotator.backupClient, addr, backupIP)
		}
	}

	log.Println("IPRotator initialized successfully.")
	return rotator, nil
}

// Fetch 实现了负载均衡的请求分发
func (r *IPRotator) Fetch(method, url string, header http.Header, body io.Reader) (*http.Response, error) {
	reqNum := atomic.AddUint64(&r.rrCounter, 1)
	slotIdx := reqNum % uint64(len(r.slots))
	selectedSlot := r.slots[slotIdx]
	return selectedSlot.execute(method, url, header, body)
}

// FetchWithRetry 带重试机制的 Fetch
// 修改点：将 5xx 错误视为需要重试的错误
func (r *IPRotator) FetchWithRetry(method, url string, header http.Header, body io.Reader) (*http.Response, error) {
	maxCnt := tools.Atoi(os.Getenv("EX_MAX_RETRY"), 3) // 默认重试3次
	var resp *http.Response
	var err error

	for cnt := 0; cnt < maxCnt; cnt++ {
		resp, err = r.Fetch(method, url, header, body)

		// --- 修改逻辑开始 ---
		// 如果网络错误，或者状态码 >= 500 (如 502)，则进行重试
		if err != nil || (resp.StatusCode >= 500) {
			status := "N/A"
			if resp != nil {
				status = strconv.Itoa(resp.StatusCode)
			}
			log.Printf("[Retry] Attempt %d/%d encountered error/status(%s), retrying...", cnt+1, maxCnt, status)

			// 稍微等待一下，给轮换一点时间
			time.Sleep(time.Duration(cnt+1) * time.Second)
			continue
		}
		// --- 修改逻辑结束 ---

		// 成功且状态码正常
		resp.Header.Add("X-Retry-Count", strconv.Itoa(cnt))
		return resp, err
	}

	// 所有重试失败，尝试备用客户端
	if r.backupClient != nil {
		log.Println("[Retry] All slots failed, using backup client.")
		resp, err = r.backupClient.Fetch(method, url, header, body)
		if resp != nil {
			resp.Header.Add("X-Retry-Count", strconv.Itoa(-1))
		}
		return resp, err
	}

	return resp, err
}

// Run 模拟运行
func Run(addr string) {
	if addr == "" {
		return
	}
	godotenv.Load(".env")

	manager, err := myfetch.NewManager(os.Getenv("IPV6_IFACE"), os.Getenv("IPV6_PREFIX"))
	if err != nil {
		log.Fatalf("Failed to create manager: %v", err)
	}

	rotator, err := NewIPRotator(manager, -1, os.Getenv("EX_BACKUP_IP"))
	if err != nil {
		log.Fatalf("Failed to create IP rotator: %v", err)
	}

	ExhProxy(rotator, addr)
}
