package proxy

import "strings"

// HostAllowlist 是代理目标 host 白名单。匹配规则（大小写不敏感、忽略端口）：
//   - "*"            → 允许任意 host（等同不限制）
//   - "*.example.com" → 允许 example.com 的所有子域（不含 apex 本身）
//   - "example.com"  → 仅精确匹配该 host
type HostAllowlist struct {
	allowAll bool
	exact    []string
	suffixes []string // 以 "." 开头，如 ".sinaimg.com"
}

// ParseHostAllowlist 解析逗号/空白/分号分隔的 host 白名单。
// 返回 nil 表示未配置（不做限制）。
func ParseHostAllowlist(spec string) *HostAllowlist {
	fields := strings.FieldsFunc(spec, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\n' || r == '\t' || r == '\r' || r == ';'
	})
	if len(fields) == 0 {
		return nil
	}
	al := &HostAllowlist{}
	for _, f := range fields {
		f = strings.ToLower(trimHostPort(f))
		f = strings.TrimSuffix(f, ".")
		if f == "" {
			continue
		}
		switch {
		case f == "*":
			al.allowAll = true
		case strings.HasPrefix(f, "*."):
			dom := strings.TrimPrefix(f, "*.")
			if dom != "" {
				al.suffixes = append(al.suffixes, "."+dom)
			}
		default:
			al.exact = append(al.exact, f)
		}
	}
	return al
}

// Match 判断 host（可带端口）是否命中白名单。
func (a *HostAllowlist) Match(host string) bool {
	if a == nil || a.allowAll {
		return true
	}
	h := strings.ToLower(trimHostPort(host))
	h = strings.TrimSuffix(h, ".")
	if h == "" {
		return false
	}
	for _, e := range a.exact {
		if h == e {
			return true
		}
	}
	for _, s := range a.suffixes {
		// 仅子域命中，不含 apex 本身
		if len(h) > len(s) && strings.HasSuffix(h, s) {
			return true
		}
	}
	return false
}

// trimHostPort 去掉 host 的 :port / [v6]:port 后缀，保留 host 本体。
func trimHostPort(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return host
	}
	// [::1]:8080
	if strings.HasPrefix(host, "[") {
		if i := strings.LastIndex(host, "]"); i >= 0 {
			return host[1:i]
		}
		return host
	}
	if i := strings.LastIndex(host, ":"); i >= 0 {
		// 仅当冒号后全是数字时视为端口（避免误伤 IPv6 裸地址）
		if isDigits(host[i+1:]) {
			return host[:i]
		}
	}
	return host
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}