package exhentai_stream

import (
	"fmt"
	"testing"
)

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
