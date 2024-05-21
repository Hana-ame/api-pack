package functions

import (
	"fmt"
	"testing"
)

func TestGetIcon(t *testing.T) {
	url := "https://moonchan.xyz/favicon.ico"
	getIcon(url)
}

func TestPostUrl(t *testing.T) {
	var a any
	a = getIconUrl("moonchan.xyz")
	fmt.Println(a)
	updateIconUrl("moonchan.xyz", "overrided..")
	a = getIconUrl("moonchan.xyz")
	fmt.Println(a)
}
