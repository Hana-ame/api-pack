// neo-moonchan @ 240801

package myhash

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
)

func Sha256(s string) string {
	hash := sha256.Sum256([]byte(s))
	hexString := hex.EncodeToString(hash[:])
	return hexString
}

func Md5(s string) string {
	hash := md5.Sum([]byte(s))
	hexString := hex.EncodeToString(hash[:])
	return hexString
}
