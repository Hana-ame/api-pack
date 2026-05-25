package tools

import (
	"fmt"
	"strconv"
	"testing"
	"time"
)

func TestTimestap(t *testing.T) {
	cookieValue := strconv.Itoa(int(NewTimeStamp()) + 65536*1000*60)
	old := NewTimeStamp()
	time.Sleep(1 * time.Second)
	new := NewTimeStamp()
	fmt.Println(new - old)
	fmt.Println(old)
	fmt.Println(cookieValue)
}
