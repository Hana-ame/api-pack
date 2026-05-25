package tools

import (
	"encoding/base64"
	"fmt"
)

// "username.timestamp.[hashvalue(username.timestamp)]"
func encode(data string) string {
	// 使用RawURLEncoding进行Base64编码，不包含填充字符
	rawEncoded := base64.RawURLEncoding.EncodeToString([]byte(data))
	fmt.Println("Encoded (without padding):", rawEncoded)

	return rawEncoded
}

// "username.timestamp.[hashvalue(username.timestamp)]"
func decode(data string) (string, error) {
	// 解码RawURLEncoding
	rawDecoded, err := base64.RawURLEncoding.DecodeString(data)
	return string(rawDecoded), err
}
