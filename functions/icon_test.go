package functions

import "testing"

func TestGetIcon(t *testing.T) {
	url := "https://moonchan.xyz/favicon.ico"
	getIcon(url)
}
