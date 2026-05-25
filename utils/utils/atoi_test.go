package tools

import (
	"fmt"
	"testing"
)

func TestAtoi(t *testing.T) {
	{
		a := Atoi("1,0_0", -1)
		fmt.Println(a)
	}
	{
		a := Atoi("aaa", -1)
		fmt.Println(a)
	}
}
