package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"os"
	"time"

	myfetch "github.com/Hana-ame/api-pack/tools/my_fetch/v2"
	tools "github.com/Hana-ame/api-pack/tools/utils"
)

func main() {
	tools.Or(1, 1)
	// 1. Setup Manager
	mgr, err := myfetch.NewManager("sit1", os.Getenv("IPV6_PREFIX"))
	if err != nil {
		panic(err)
	}

	// 2. Generate and Add IP
	ip, err := mgr.GenerateIP()
	if err != nil {
		panic(err)
	}

	fmt.Printf("[*] Using Local IP: %s\n", ip.String())
	err = mgr.AddAddr(ip)
	if err != nil {
		panic(fmt.Errorf("failed to add IP: %w", err))
	}

	// 3. IMPORTANT: Cleanup on exit
	defer func() {
		fmt.Printf("[*] Cleaning up: Removing IP %s\n", ip.String())
		mgr.DelAddr(ip)
	}()

	// 4. THE "TENTATIVE" FIX:
	// When you add an IPv6, Linux performs DAD (Duplicate Address Detection).
	// The IP is unusable for ~1-2 seconds. Without this sleep, you will get
	// a "bind: cannot assign requested address" or a long timeout.
	fmt.Println("[*] Waiting 2s for IPv6 DAD...")
	time.Sleep(2 * time.Second)

	// 5. Your specific Client Configuration
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				dialer := &net.Dialer{
					LocalAddr: &net.TCPAddr{
						IP: net.IP(ip.AsSlice()), // Convert netip.Addr to net.IP
					},
					Timeout:   15 * time.Second,
					KeepAlive: 30 * time.Second,
					Resolver: &net.Resolver{
						PreferGo: true,
						Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
							return net.Dial("udp", "1.1.1.1:53")
						},
					},
				}
				return dialer.DialContext(ctx, network, addr)
			},

			TLSClientConfig: &tls.Config{
				ServerName: "exhentai.org",
			},

			MaxIdleConns:        100,
			IdleConnTimeout:     15 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// 6. Trace and Request
	req, _ := http.NewRequest("GET", "https://exhentai.org", nil)

	trace := &httptrace.ClientTrace{
		DNSStart:             func(_ httptrace.DNSStartInfo) { fmt.Printf("%-20s %v\n", "DNS Start:", time.Now()) },
		ConnectStart:         func(_, _ string) { fmt.Printf("%-20s %v\n", "Connect Start:", time.Now()) },
		ConnectDone:          func(_, _ string, err error) { fmt.Printf("%-20s %v (err: %v)\n", "Connect Done:", time.Now(), err) },
		TLSHandshakeStart:    func() { fmt.Printf("%-20s %v\n", "TLS Start:", time.Now()) },
		TLSHandshakeDone:     func(_ tls.ConnectionState, err error) { fmt.Printf("%-20s %v\n", "TLS Done:", time.Now()) },
		GotFirstResponseByte: func() { fmt.Printf("%-20s %v\n", "First Byte:", time.Now()) },
	}

	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))

	fmt.Println("--- Starting Request ---")
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Request Failed: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("--- Finished in %v ---\n", time.Since(start))
	fmt.Printf("Status: %s, Body Length: %d\n", resp.Status, len(body))
}
