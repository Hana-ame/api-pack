package main

import (
	"fmt"
	"testing"
)

type A struct {
	A int
}

type B struct {
	A
	B int
}

type IA interface {
	
}

func TestXxx(t *testing.T) {
	a := A{}
	fmt.Println(a)

	b := B{}
	fmt.Println(b)

	f := func(ia interface{}) {
		fmt.Println(ia)
	}

	f(a)
	f(b)

}
