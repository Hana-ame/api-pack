package tools

import (
	"strconv"
	"strings"
)

// exclude non-digital chars
func Atoi(s string, defaultValue int) int {
	stringBuilder := &strings.Builder{}
	for _, c := range s {
		if c >= '0' && c <= '9' {
			stringBuilder.WriteRune(c)
		}
	}
	n, err := strconv.Atoi(stringBuilder.String())
	if err != nil {
		return defaultValue
	}
	return n
}
