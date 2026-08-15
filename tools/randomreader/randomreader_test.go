package randomreader

import (
	"fmt"
	"testing"
)

func TestRandom(t *testing.T) {
	p := make([]byte, 32)
	DefaultRandomReader.Read(p)
	fmt.Println(string(p))
	DefaultRandomReader.Read(p)
	fmt.Println(string(p))
	DefaultRandomReader.Read(p)
	fmt.Println(string(p))
	DefaultRandomReader.Read(p)
	fmt.Println(string(p))
}
