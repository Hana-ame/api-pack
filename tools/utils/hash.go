// azure-go @ 2023-12-21
// antenna @ 2024-01-05

package tools

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

func Hash(s ...string) string {
	hash := sha256.Sum256([]byte(strings.Join(s, "")))
	hashString := fmt.Sprintf("%x", hash)
	return hashString
}
