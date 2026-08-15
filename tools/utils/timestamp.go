package tools

import (
	"sync"
	"time"
)

// func Now() int64 {
// 	ts := (((time.Now().UnixNano()) / 100_000) << 16) / 10
// 	return (int64(ts))
// }

var mu sync.Mutex
var lastTime int64
var lastSequence int64

// NewTimeStamp 生成一个唯一的时间戳 ID，保证同一毫秒内唯一
// 65536000 = 1s
func NewTimeStamp() int64 {
	mu.Lock()
	defer mu.Unlock()
	now := time.Now().UnixMilli() << 16

	if now == lastTime {
		lastSequence++
	} else {
		lastTime = now
		// if lastSequence < 65536 {
		lastSequence = 0
		// } else {
		// 	lastSequence -= 65536
		// }
	}
	return now + lastSequence
}

// GetTimestamp 获取纳秒级时间戳的int64形式
// 请用NewTimestamp
func GetTimestamp() int64 {
	return int64(float64(time.Now().UnixNano()) * (float64(65536) / float64(1_000_000)))
}

// GetTimestampSeconds 获取秒级时间戳
func GetTimestampSeconds() int64 {
	return time.Now().Unix()
}

// GetTimestampMilliseconds 获取毫秒级时间戳
func GetTimestampMilliseconds() int64 {
	return time.Now().UnixNano() / 1e6
}

// GetTimestampMicroseconds 获取微秒级时间戳
func GetTimestampMicroseconds() int64 {
	return time.Now().UnixNano() / 1e3
}

// GetTimestampNanoseconds 获取纳秒级时间戳
func GetTimestampNanoseconds() int64 {
	return time.Now().UnixNano()
}
