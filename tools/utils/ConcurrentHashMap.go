package tools

import "sync"

// ConcurrentHashMap 是一个线程安全的映射，支持泛型类型

type ConcurrentHashMap[K comparable, V any] struct {
	m map[K]V
	sync.RWMutex
}

// NewConcurrentHashMap 创建一个新的 ConcurrentHashMap 实例
func NewConcurrentHashMap[K comparable, V any]() *ConcurrentHashMap[K, V] {
	return &ConcurrentHashMap[K, V]{
		m:       make(map[K]V),
		RWMutex: sync.RWMutex{},
	}
}

func (m *ConcurrentHashMap[K, V]) Contains(key K) bool {
	m.RLock()
	defer m.RUnlock()
	_, ok := m.m[key]
	return ok
}

// Get 根据键获取值
func (m *ConcurrentHashMap[K, V]) Get(key K) (V, bool) {
	m.RLock()
	defer m.RUnlock()
	v, ok := m.m[key]
	return v, ok
}

func (m *ConcurrentHashMap[K, V]) GetOrDefault(key K, defaultValue V) V {
	m.RLock()
	defer m.RUnlock()
	v, ok := m.m[key]
	if !ok {
		return defaultValue
	}
	return v
}

// Put 插入键值对
func (m *ConcurrentHashMap[K, V]) Put(key K, value V) *ConcurrentHashMap[K, V] {
	m.Lock()
	defer m.Unlock()
	m.m[key] = value
	return m
}

// ture = 插入成功,  false = 插入失败
func (m *ConcurrentHashMap[K, V]) PutIfAbsent(key K, value V) bool {
	m.Lock()
	defer m.Unlock()
	if _, exists := m.m[key]; exists {
		return false
	}
	m.m[key] = value
	return true
}

// Remove 根据键删除元素
func (m *ConcurrentHashMap[K, V]) Remove(key K) {
	m.Lock()
	defer m.Unlock()
	delete(m.m, key)
}

// ForEach 遍历映射并对每个键值对调用处理函数
func (m *ConcurrentHashMap[K, V]) ForEach(handler func(key K, value V)) {
	if handler == nil {
		return
	}
	m.RLock()
	defer m.RUnlock()
	for key, value := range m.m {
		handler(key, value)
	}
}

// Size 返回映射的大小
func (m *ConcurrentHashMap[K, V]) Size() int {
	m.RLock()
	defer m.RUnlock()
	return len(m.m)
}

// ConcurrentHashSet 是一个线程安全的集合，通过继承 ConcurrentHashMap 实现
type ConcurrentHashSet[K comparable] struct {
	ConcurrentHashMap[K, struct{}]
}

// NewConcurrentHashSet 创建一个新的 ConcurrentHashSet 实例
func NewConcurrentHashSet[K comparable]() *ConcurrentHashSet[K] {
	return &ConcurrentHashSet[K]{
		ConcurrentHashMap: *NewConcurrentHashMap[K, struct{}](),
	}
}

// Add 向集合中添加元素
func (s *ConcurrentHashSet[K]) Add(key K) {
	s.Put(key, struct{}{})
}

// ForEach 遍历集合并对每个元素调用处理函数
func (s *ConcurrentHashSet[K]) ForEach(handler func(key K)) {
	if handler == nil {
		return
	}
	s.ConcurrentHashMap.ForEach(func(key K, _ struct{}) {
		handler(key)
	})
}
