// 曾经在azure用过，现在用LRU了
// deprecated@24.11.13

package tools

import (
	"sync"
	"sync/atomic"
)

type SyncMapWithCount struct {
	*atomic.Int32
	*sync.Map
}

func NewSyncMapWithCount() *SyncMapWithCount {
	return &SyncMapWithCount{
		&atomic.Int32{},
		&sync.Map{},
	}
}
func (s *SyncMapWithCount) Len() int {
	return int(s.Int32.Load())
}
func (s *SyncMapWithCount) Store(key any, value any) {
	s.Map.Store(key, value)
	s.Int32.Add(1)
}
func (s *SyncMapWithCount) Load(key any) (value any, ok bool) {
	return s.Map.Load(key)
}
func (s *SyncMapWithCount) Delete(key any) {
	s.LoadAndDelete(key)
}
func (s *SyncMapWithCount) LoadAndDelete(key any) (value any, loaded bool) {
	value, loaded = s.Map.LoadAndDelete(key)
	if loaded {
		s.Int32.Add(-1)
	}
	return
}
func (s *SyncMapWithCount) CompareAndDelete(key, old any) (deleted bool) {
	deleted = s.Map.CompareAndDelete(key, old)
	if deleted {
		s.Int32.Add(-1)
	}
	return
}
