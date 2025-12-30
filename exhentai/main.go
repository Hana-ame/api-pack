package exhentai

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	myfetch "github.com/Hana-ame/api-pack/tools/my_fetch/v2"
)

type client struct {
	*myfetch.Client // 实际的 http client
	*netip.Addr     // 绑定的 IP，用于日志和清理
}

const (
	// DefaultRotationThreshold 默认轮换阈值
	DefaultRotationThreshold = 500
	// DefaultCleanupDelay 删除旧 IP 的延迟时间
	DefaultCleanupDelay = 6 * time.Second
)

// IPRotator 负责管理和轮换绑定了IP的http客户端
type IPRotator struct {
	ipManager *myfetch.Manager
	threshold int64

	// currentClientHolder 用于原子存储当前的 *client
	// 这样读取时完全无锁
	currentClientHolder atomic.Value

	// requestCounter 原子计数器
	requestCounter int64

	// rotationMu 仅在轮换操作时使用，不阻塞普通请求
	rotationMu sync.Mutex
	// nextClient 受 rotationMu 保护
	nextClient *client
}

// NewIPRotator 创建并初始化一个IPRotator
func NewIPRotator(manager *myfetch.Manager) (*IPRotator, error) {
	rotator := &IPRotator{
		ipManager: manager,
		threshold: DefaultRotationThreshold,
	}

	// 1. 初始化当前 client
	current, err := rotator.prepareNewClient()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize current client: %w", err)
	}
	// 存入原子容器
	rotator.currentClientHolder.Store(current)

	// 2. 提前准备下一个 client
	next, err := rotator.prepareNewClient()
	if err != nil {
		_ = rotator.ipManager.DelAddr(*(current.Addr))
		return nil, fmt.Errorf("failed to initialize next client: %w", err)
	}
	rotator.nextClient = next

	log.Printf("IPRotator initialized. Current IP: %s, Next IP: %s\n",
		current.Addr, next.Addr)

	return rotator, nil
}

// prepareNewClient 生成 IP 并创建 Client (这是一个耗时操作)
func (r *IPRotator) prepareNewClient() (*client, error) {
	ip, err := r.ipManager.GenerateIP()
	if err != nil {
		return nil, fmt.Errorf("failed to generate IP: %w", err)
	}

	r.ipManager.AddAddr(ip)

	c := &client{
		Addr: &ip,
		Client: &myfetch.Client{
			Client: &http.Client{
				Transport: &http.Transport{
					DialContext: (&net.Dialer{
						LocalAddr: &net.TCPAddr{
							IP: ip.AsSlice(),
						},
						Timeout:   30 * time.Second,
						KeepAlive: 30 * time.Second,
					}).DialContext,
					MaxIdleConns:        100,
					IdleConnTimeout:     90 * time.Second,
					TLSHandshakeTimeout: 10 * time.Second,
				},
				CheckRedirect: func(req *http.Request, via []*http.Request) error {
					return http.ErrUseLastResponse
				},
			},
		},
	}
	log.Printf("Prepared new client with IP: %s\n", ip)
	return c, nil
}

// getCurrentClient 安全地获取当前客户端
func (r *IPRotator) getCurrentClient() *client {
	return r.currentClientHolder.Load().(*client)
}

// Fetch 执行HTTP请求，核心路径无锁
func (r *IPRotator) Fetch(method, url string, header http.Header, body io.Reader) (*http.Response, error) {
	// 1. 【无锁】获取当前 Client
	// atomic.Load 非常快，纳秒级
	current := r.getCurrentClient()

	// 2. 【无锁】增加计数器
	// atomic.Add 也是纳秒级
	count := atomic.AddInt64(&r.requestCounter, 1)

	// 3. 检查阈值
	if count >= r.threshold {
		// 尝试触发轮换。
		// 使用 TryLock：如果拿不到锁，说明已经有别的 goroutine 在处理轮换了，
		// 此时我们不做等待，直接使用当前的（旧）client 发送请求即可。
		// 这样彻底避免了阻塞。
		if r.rotationMu.TryLock() {
			// 获取到锁的这个 goroutine 负责执行切换逻辑
			// 注意：这里传入 current 是为了稍后做清理
			r.performRotation(current)
			// performRotation 内部会解锁，并启动后台任务

			// 轮换完成后，为了防止瞬间使用刚换下来的旧IP（虽然通常没问题），
			// 我们可以再次获取一下最新的（虽然大概率还是刚才那个，除非切换极其快）
			current = r.getCurrentClient()
		}
	}

	// 4. 执行请求
	return current.Fetch(method, url, header, body)
}

// performRotation 执行切换逻辑，必须在持有 rotationMu 时调用
func (r *IPRotator) performRotation(oldClient *client) {
	// 检查 nextClient 是否可用
	if r.nextClient == nil {
		// 这种情况一般是因为生成 IP 太慢，还没准备好
		// 放弃本次轮换，解锁，继续用旧的跑
		r.rotationMu.Unlock()
		// 重置计数器一部分，避免每个请求都进来 tryLock
		// 比如减去 100，让它过一会再试
		atomic.AddInt64(&r.requestCounter, -100)
		log.Println("Next client not ready yet, skipping rotation.")
		return
	}

	next := r.nextClient

	// 1. 切换指针：将 current 替换为 next
	r.currentClientHolder.Store(next)

	// 2. 将内部 next 置空，标记需要补充
	r.nextClient = nil

	// 3. 重置计数器
	// 直接归零。虽然此时可能有其他并发请求让它变成了 1005，但归零是安全的
	atomic.StoreInt64(&r.requestCounter, 0)

	// 4. 解锁（关键操作已完成）
	r.rotationMu.Unlock()

	log.Printf("Rotated IP from %s to %s", oldClient.Addr, next.Addr)

	// 5. 【后台】补充新的 nextClient 和 清理旧 client
	go r.backgroundTask(oldClient)
}

// backgroundTask 在后台补充新IP并清理旧IP
func (r *IPRotator) backgroundTask(oldClient *client) {
	// A. 准备下一个备用 Client
	// 这步比较耗时，所以放在后台
	newNext, err := r.prepareNewClient()
	if err != nil {
		log.Printf("CRITICAL: Failed to prepare new next client: %v", err)
		// 如果这里失败了，下次轮换时 nextClient 就是 nil，会触发上面的 skip 逻辑
	} else {
		// 只有在写入 nextClient 字段时才需要锁
		r.rotationMu.Lock()
		r.nextClient = newNext
		r.rotationMu.Unlock()
		log.Printf("New backup client ready: %s", newNext.Addr)
	}

	// B. 延迟清理旧 Client
	time.Sleep(DefaultCleanupDelay)

	log.Printf("Cleaning up old IP: %s", oldClient.Addr)
	// 关闭旧客户端的空闲连接（可选，视 myfetch 实现而定）
	oldClient.Client.CloseIdleConnections()

	if err := r.ipManager.DelAddr(*oldClient.Addr); err != nil {
		log.Printf("Error deleting IP %s: %v", oldClient.Addr, err)
	}
}

// Run 模拟运行
func Run(addr string) {
	if addr == "" {
		return
	}

}
