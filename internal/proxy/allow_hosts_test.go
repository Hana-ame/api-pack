package proxy

import "testing"

func TestHostAllowlistMatch(t *testing.T) {
	al := ParseHostAllowlist("*.sinaimg.com,ehgt.org")
	if al == nil {
		t.Fatal("expected non-nil allowlist")
	}
	cases := []struct {
		host string
		want bool
	}{
		{"wx1.sinaimg.com", true},
		{"wx2.sinaimg.com", true},
		{"a.b.sinaimg.com", true},
		{"wx1.sinaimg.com:443", true},
		{"WX1.SINAIMG.COM", true},
		{"sinaimg.com", false},          // 通配不含 apex
		{"ehgt.org", true},              // 精确匹配
		{"ehgt.org:8443", true},         // 忽略端口
		{"www.ehgt.org", false},         // 未配置
		{"evil.com", false},             // 不在白名单
		{"sinaimg.com.evil.com", false}, // 后缀伪造
		{"", false},
	}
	for _, c := range cases {
		if got := al.Match(c.host); got != c.want {
			t.Errorf("Match(%q)=%v, want %v", c.host, got, c.want)
		}
	}
}

func TestHostAllowlistAllowAll(t *testing.T) {
	al := ParseHostAllowlist("*")
	if !al.Match("anything.example") {
		t.Error(`"*" 应放开任意 host`)
	}
}

func TestHostAllowlistNil(t *testing.T) {
	var al *HostAllowlist
	if !al.Match("anything") {
		t.Error("nil allowlist 应不限制")
	}
	if ParseHostAllowlist("") != nil {
		t.Error("空配置应返回 nil")
	}
}

// TestHostAllowlistEmptyRejectsAll 锁定"默认拒绝一切"安全默认:
// 空 allowlist（&HostAllowlist{}）必须不匹配任何 host, 而不是像 nil 那样放行。
// 这是 app.go 里 PROXY_ALLOW_HOSTS 未配置时的默认行为。
func TestHostAllowlistEmptyRejectsAll(t *testing.T) {
	al := &HostAllowlist{}
	cases := []string{"wx1.sinaimg.com", "ehgt.org", "github.com", "raw.githubusercontent.com", "evil.com", "localhost"}
	for _, h := range cases {
		if al.Match(h) {
			t.Errorf("空 allowlist 应拒绝 %q, 却放行了", h)
		}
	}
}
