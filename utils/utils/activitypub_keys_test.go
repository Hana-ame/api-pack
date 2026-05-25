package tools

import (
	"fmt"
	"testing"
)

func TestGenerate(t *testing.T) {
	pk, _ := GeneratePrivateKey()
	pkpem, _ := MarshalPrivateKey(pk)
	pubpem, _ := MarshalPublicKey(&pk.PublicKey)
	fmt.Println(string(pkpem))
	fmt.Println(string(pubpem))
}
