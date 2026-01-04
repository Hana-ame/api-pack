package exhentai

import "strings"

func containsPrefix(prefixSlice []string, v string) bool {
	for _, prefix := range prefixSlice {
		if strings.HasPrefix(v, prefix) {
			return true
		}
	}
	return false
}
