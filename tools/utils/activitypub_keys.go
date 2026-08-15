package tools

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"strconv"
)

// @deprecated
func DefaultValue[T any](f func() (T, error), defaultValue T) T {
	if v, e := f(); e == nil {
		return v
	}
	return defaultValue
}

var RSABits = DefaultValue(func() (int, error) { return strconv.Atoi(os.Getenv("RSA_BITS")) }, 4096)

// var RSABits = func() int {
// 	bits, err := strconv.Atoi(os.Getenv("RSA_BITS"))
// 	if err != nil {
// 		return 4096
// 	}
// 	return bits
// }()

// create a new private key
func GeneratePrivateKey() (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, RSABits)
}

// marshal private key to pem format
func MarshalPrivateKey(privateKey *rsa.PrivateKey) (privateKeyPem []byte, err error) {
	bytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return bytes, err
	}
	privateKeyPem = pem.EncodeToMemory(
		&pem.Block{
			Type:  "PRIVATE KEY",
			Bytes: bytes,
		},
	)
	return
}

// marshal public key to pem format
func MarshalPublicKey(publicKey *rsa.PublicKey) (publicKeyPem []byte, err error) {
	bytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return bytes, err
	}
	publicKeyPem = pem.EncodeToMemory(
		&pem.Block{
			Type:  "PUBLIC KEY",
			Bytes: bytes,
		},
	)
	return
}

// parse pem string to public key
func ParsePublicKey(publicKeyPem []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(publicKeyPem)
	pkRaw, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	pk, ok := pkRaw.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("type is not *rsa.PublicKey: %v", pk)
	}
	return pk, nil
}

// parse pem string to private key
func ParsePrivateKey(privateKeyPem []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(privateKeyPem)
	pkRaw, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	pk, ok := pkRaw.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("type is not *rsa.PrivateKey: %v", pk)
	}
	return pk, nil
}

func ReadKeyFromFile(fn string) (*rsa.PrivateKey, error) {
	pem, err := os.ReadFile(fn)
	if err != nil {
		return nil, err
	}
	return ParsePrivateKey(pem)
}
