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
	"golang.org/x/net/http2" // Added for H2 support
)

// Context key for passing the fingerprint down to the dialer
type contextKey string

const fingerprintKey contextKey = "tls_fingerprint"

const (
	DefaultRotationThreshold = 1000
	DefaultCleanupDelay      = 6 * time.Second
	DefaultPoolSize          = 5
)

type client struct {
	*myfetch.Client
	Addr *netip.Addr
}

// utlsDialer handles the custom TCP dial + uTLS handshake
type utlsDialer struct {
	targetAddr  string
	localIP     net.IP
	netDialer   *net.Dialer
	fingerprint tls.ClientHelloID
}

func (d *utlsDialer) DialTLSContext(ctx context.Context, network, addr string) (net.Conn, error) {
	// 1. Get the desired fingerprint from context, fallback to default
	id, ok := ctx.Value(fingerprintKey).(tls.ClientHelloID)
	if !ok {
		id = d.fingerprint
	}

	// 2. Establish TCP connection bound to local IP
	d.netDialer.LocalAddr = &net.TCPAddr{IP: d.localIP}
	tcpConn, err := d.netDialer.DialContext(ctx, "tcp6", d.targetAddr)
	if err != nil {
		return nil, fmt.Errorf("TCP dial failed: %w", err)
	}

	// 3. Prepare uTLS Config
	config := &tls.Config{
		ServerName: "exhentai.org",
		// CRITICAL: ALPN must include h2 for modern browsers
		NextProtos: []string{"h2", "http/1.1"},
	}

	uConn := tls.UClient(tcpConn, config, id)

	// 4. Handshake
	if err := uConn.HandshakeContext(ctx); err != nil {
		tcpConn.Close()
		return nil, fmt.Errorf("uTLS handshake failed: %w", err)
	}

	return uConn, nil
}

type clientSlot struct {
	id        int
	ipManager *myfetch.Manager

	currentClientHolder atomic.Value
	usageCounter        int64

	rotationMu sync.Mutex
	nextClient *client

	targetAddr         string
	defaultFingerprint tls.ClientHelloID
}

func newClientSlot(id int, manager *myfetch.Manager, targetAddr string, fingerprint tls.ClientHelloID) (*clientSlot, error) {
	slot := &clientSlot{
		id:                 id,
		ipManager:          manager,
		targetAddr:         targetAddr,
		defaultFingerprint: fingerprint,
	}

	current, err := slot.prepareNewClient()
	if err != nil {
		return nil, err
	}
	slot.currentClientHolder.Store(current)

	next, err := slot.prepareNewClient()
	if err != nil {
		_ = slot.ipManager.DelAddr(*(current.Addr))
		return nil, err
	}
	slot.nextClient = next

	return slot, nil
}

func (s *clientSlot) prepareNewClient() (*client, error) {
	ip, err := s.ipManager.GenerateIP()
	if err != nil {
		return nil, fmt.Errorf("slot %d generate ip failed: %w", s.id, err)
	}

	if err := s.ipManager.AddAddr(ip); err != nil {
		return nil, fmt.Errorf("slot %d add addr failed: %w", s.id, err)
	}

	dialer := &utlsDialer{
		targetAddr: s.targetAddr,
		localIP:    ip.AsSlice(),
		netDialer: &net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		},
		fingerprint: s.defaultFingerprint,
	}

	// Define transport
	tr := &http.Transport{
		DialTLSContext:      dialer.DialTLSContext,
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		// Fallback for non-TLS if needed
		DialContext: (&net.Dialer{
			LocalAddr: &net.TCPAddr{IP: ip.AsSlice()},
			Timeout:   10 * time.Second,
		}).DialContext,
	}

	// CRITICAL: Configure HTTP/2 support. This fixes the "malformed response" error.
	if err := http2.ConfigureTransport(tr); err != nil {
		return nil, fmt.Errorf("failed to configure h2: %w", err)
	}

	c := &client{
		Addr: &ip,
		Client: &myfetch.Client{
			Client: &http.Client{
				Transport: tr,
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

func getFingerprintFromUA(ua string) tls.ClientHelloID {
	ua = strings.ToLower(ua)
	if strings.Contains(ua, "firefox") {
		return tls.HelloFirefox_Auto
	}
	if strings.Contains(ua, "iphone") || strings.Contains(ua, "ipad") {
		return tls.HelloIOS_Auto
	}
	if strings.Contains(ua, "android") {
		return tls.HelloAndroid_11_OkHttp
	}
	return tls.HelloChrome_Auto
}

// DoProxyRequest handles incoming proxy requests from a real user
func (s *clientSlot) DoProxyRequest(userReq *http.Request) (*http.Response, error) {
	ua := userReq.Header.Get("User-Agent")
	fingerprint := getFingerprintFromUA(ua)

	// Attach fingerprint to context for the dialer
	ctx := context.WithValue(userReq.Context(), fingerprintKey, fingerprint)

	// Ensure absolute URL for the proxy target
	targetURL := userReq.URL.String()
	if !strings.HasPrefix(targetURL, "http") {
		targetURL = "https://exhentai.org" + targetURL
	}

	proxyReq, err := http.NewRequestWithContext(ctx, userReq.Method, targetURL, userReq.Body)
	if err != nil {
		return nil, err
	}

	// Copy headers and clean proxy/hop-by-hop headers
	for k, vv := range userReq.Header {
		lowKey := strings.ToLower(k)
		if lowKey == "connection" || lowKey == "proxy-connection" || lowKey == "keep-alive" {
			continue
		}
		for _, v := range vv {
			proxyReq.Header.Add(k, v)
		}
	}

	current := s.getCurrentClient()
	return current.Client.Do(proxyReq)
}

// execute is used for internal programmed requests
func (s *clientSlot) execute(method, url string, header http.Header, body io.Reader) (*http.Response, error) {
	current := s.getCurrentClient()

	count := atomic.AddInt64(&s.usageCounter, 1)
	if count >= DefaultRotationThreshold {
		if s.rotationMu.TryLock() {
			s.performRotation(current)
			current = s.getCurrentClient()
		}
	}

	ua := header.Get("User-Agent")
	fingerprint := getFingerprintFromUA(ua)
	ctx := context.WithValue(context.Background(), fingerprintKey, fingerprint)

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header = header

	return current.Client.Do(req)
}

func (s *clientSlot) performRotation(oldClient *client) {
	if s.nextClient == nil {
		s.rotationMu.Unlock()
		atomic.AddInt64(&s.usageCounter, -100)
		return
	}

	next := s.nextClient
	s.currentClientHolder.Store(next)
	s.nextClient = nil
	atomic.StoreInt64(&s.usageCounter, 0)
	s.rotationMu.Unlock()

	log.Printf("[Slot %d] Rotated: %s -> %s", s.id, oldClient.Addr, next.Addr)
	go s.backgroundPrepare(oldClient)
}

func (s *clientSlot) backgroundPrepare(oldClient *client) {
	newNext, err := s.prepareNewClient()
	if err != nil {
		log.Printf("[Slot %d] ERROR preparing next: %v", s.id, err)
	} else {
		s.rotationMu.Lock()
		s.nextClient = newNext
		s.rotationMu.Unlock()
		log.Printf("[Slot %d] Backup ready: %s", s.id, newNext.Addr)
	}

	time.Sleep(DefaultCleanupDelay)
	oldClient.Client.CloseIdleConnections()
	if err := s.ipManager.DelAddr(*(oldClient.Addr)); err != nil {
		log.Printf("[Slot %d] cleanup error: %v", s.id, err)
	}
}

type IPRotator struct {
	slots     []*clientSlot
	rrCounter uint64
}

func NewIPRotator(manager *myfetch.Manager, poolSize int, targetAddr string, fingerprint tls.ClientHelloID) (*IPRotator, error) {
	if poolSize <= 0 {
		poolSize = DefaultPoolSize
	}
	if targetAddr == "" {
		targetAddr = "exhentai.org:443"
	}

	rotator := &IPRotator{
		slots: make([]*clientSlot, poolSize),
	}

	log.Printf("Initializing IPRotator with pool size: %d, target: %s...", poolSize, targetAddr)

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
		return nil, fmt.Errorf("failed to init slots: %w", initErr)
	}

	log.Println("IPRotator initialized successfully.")
	return rotator, nil
}

func (r *IPRotator) Fetch(method, url string, header http.Header, body io.Reader) (*http.Response, error) {
	reqNum := atomic.AddUint64(&r.rrCounter, 1)
	slotIdx := reqNum % uint64(len(r.slots))
	return r.slots[slotIdx].execute(method, url, header, body)
}

func (r *IPRotator) DoProxyRequest(userReq *http.Request) (*http.Response, error) {
	reqNum := atomic.AddUint64(&r.rrCounter, 1)
	slotIdx := reqNum % uint64(len(r.slots))
	return r.slots[slotIdx].DoProxyRequest(userReq)
}

func Run(addr string) {
	if addr == "" {
		return
	}
	godotenv.Load(".env")

	// Ensure sit1 is configured with MTU 1280 if it's a tunnel
	manager, err := myfetch.NewManager("sit1", "")
	if err != nil {
		log.Fatalf("Failed to create manager: %v", err)
	}

	rotator, err := NewIPRotator(manager, 10, "exhentai.org:443", tls.HelloChrome_Auto)
	if err != nil {
		log.Fatalf("Failed to create IP rotator: %v", err)
	}

	// This function must be defined in your proxy entry point
	ExhProxy(rotator, addr)
}
