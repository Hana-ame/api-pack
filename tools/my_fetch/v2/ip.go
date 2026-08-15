package myfetch

import (
	"crypto/rand"
	"fmt"
	"net/netip"
	"os"
	"os/exec"
)

type Manager struct {
	InterfaceName string
	Prefix        netip.Prefix
}

// NewManager 初始化管理器，例如 prefixStr 为 "2001:470:c:6c::/64"
func NewManager(iface string, prefixStr string) (*Manager, error) {
	if prefixStr == "" {
		prefixStr = os.Getenv("IPV6_PREFIX")
		if prefixStr == "" {
			prefixStr = "2001:470:c:6c::/64" // 默认值
		}
	}
	if iface == "" {
		iface = "sit1" // 默认值
	}

	prefix, err := netip.ParsePrefix(prefixStr)
	if err != nil {
		return nil, fmt.Errorf("invalid prefix: %w", err)
	}

	return &Manager{
		InterfaceName: iface,
		Prefix:        prefix,
	}, nil
}

// GenerateIP 生成该前缀下的一个合法随机 IPv6 地址
func (m *Manager) GenerateIP() (netip.Addr, error) {
	prefixBytes := m.Prefix.Addr().As16()
	maskBits := m.Prefix.Bits()

	// 获取随机后缀字节数 (128 - maskBits) / 8
	randomBytesNeeded := (128 - maskBits) / 8
	randomPart := make([]byte, randomBytesNeeded)
	if _, err := rand.Read(randomPart); err != nil {
		return netip.Addr{}, err
	}

	// 组合前缀和随机部分
	// 注意：这里简单处理 64 位前缀的情况，如果前缀不是 8 的倍数需要位运算
	newIPBytes := prefixBytes
	for i := 0; i < randomBytesNeeded; i++ {
		newIPBytes[15-i] = randomPart[len(randomPart)-1-i]
	}

	return netip.AddrFrom16(newIPBytes), nil
}

// AddAddr 在网卡上添加 IP
func (m *Manager) AddAddr(ip netip.Addr) error {
	// 格式: ip addr add 2001:db8::1/64 dev sit1
	fullAddr := fmt.Sprintf("%s/%d", ip.String(), m.Prefix.Bits())
	cmd := exec.Command("/usr/sbin/ip", "addr", "add", fullAddr, "dev", m.InterfaceName)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to add ip %s: %s, %w", fullAddr, string(output), err)
	}
	return nil
}

// DelAddr 在网卡上移除 IP
func (m *Manager) DelAddr(ip netip.Addr) error {
	fullAddr := fmt.Sprintf("%s/%d", ip.String(), m.Prefix.Bits())
	cmd := exec.Command("/usr/sbin/ip", "addr", "del", fullAddr, "dev", m.InterfaceName)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to del ip %s: %s, %w", fullAddr, string(output), err)
	}
	return nil
}
