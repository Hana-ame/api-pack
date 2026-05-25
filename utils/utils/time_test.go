package tools

import (
	"testing"
	"time"
)

func TestTimestamp(t *testing.T) {
	// a := Now()
	// fmt.Println(a)
}

func TestTimestampForManyTimes(t *testing.T) {
	for i := 0; i < 20; i++ {
		time.Sleep(time.Microsecond * 100)
		t.Run("timestamp", TestTimestamp)
	}
}
