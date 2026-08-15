package mytypes

import (
	"fmt"
	"testing"
)

func TestCompare(t *testing.T) {
	r := Int(Int16(123)).GTE(Int(Int32(1123)))
	fmt.Println(r)
}
