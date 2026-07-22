package exhentai

import (
	"fmt"
	"net/url"
	"strings"
	"testing"
)

func TestTargetPath(t *testing.T) {
	tests := []struct {
		name       string
		targetURL  string
		wantPrefix string
	}{
		{"full URL", "https://exhentai.org/torrent/4070144/7621193/995418c9ed0.torrent", "/torrent/"},
		{"full URL z", "https://exhentai.org/z/0381/x.css", "/z/"},
		{"full URL api", "https://exhentai.org/api.php", "/api.php"},
		{"just path", "/torrent/4070144/7621193/995418c9ed0.torrent", "/torrent/"},
		{"just path z", "/z/0381/x.css", "/z/"},
		{"just path api", "/api.php", "/api.php"},
		{"full URL no match", "https://exhentai.org/g/123456/abcdef/", "/g/"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, _ := url.Parse(tt.targetURL)
			targetPath := tt.targetURL
			if u != nil {
				targetPath = u.Path
			}
			matched := strings.HasPrefix(targetPath, tt.wantPrefix)
			wantMatch := strings.HasPrefix(tt.wantPrefix, "/")
			if matched != wantMatch {
				t.Errorf("targetURL=%q targetPath=%q HasPrefix(%q)=%v, want %v", tt.targetURL, targetPath, tt.wantPrefix, matched, wantMatch)
			}
		})
	}
}

func TestStrip(t *testing.T) {
	// Simulating a response body from the original site
	originalBody := []byte(`
		<html>
			<body>
				<h1>Proxy Site</h1>
				<script defer src="https://static.cloudflareinsights.com/beacon.min.js/vcd..." data-cf-beacon='{"token": "123"}'></script>
				<p>Rest of the content</p>
			</body>
		</html>
	`)

	cleanBody := stripCloudflareBeacon(originalBody)

	// Result
	fmt.Printf("%s\n", cleanBody)
}
