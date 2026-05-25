package mytypes

type node[T any] struct {
	val  T
	prev *node[T]
	next *node[T]
}

type LinkedList[T any] struct {
	len   int
	first *node[T]
}

func NewLinedList[T any]() *LinkedList[T] {
	return &LinkedList[T]{}
}

func (l *LinkedList[T]) Len() int {
	return l.len
}

func (l *LinkedList[T]) AddLast(val T) bool {
	if l.first == nil {
		l.first = &node[T]{val: val}
		l.first.next = l.first
		l.first.prev = l.first
		l.len++
		return true
	}
	nn := &node[T]{val: val, next: l.first, prev: l.first.prev}
	l.first.prev.next = nn
	l.first.prev = nn
	l.len++
	return true
}

func (l *LinkedList[T]) AddFirst(val T) bool {
	if l.first == nil {
		l.first = &node[T]{val: val}
		l.first.next = l.first
		l.first.prev = l.first
		l.len++
		return true
	}
	nn := &node[T]{val: val, next: l.first, prev: l.first.prev}
	l.first.prev.next = nn
	l.first.prev = nn
	l.first = nn
	l.len++
	return true
}

func (l *LinkedList[T]) RemoveFirst() (T, bool) {
	if l.len == 0 || l.first == nil {
		var e T
		return e, false
	}
	if l.len == 1 {
		e := l.first.val
		l.first = nil
		l.len--
		return e, true
	}
	rn := l.first
	l.first = rn.next
	rn.prev.next = rn.next
	rn.next.prev = rn.prev
	l.len--
	return rn.val, true
}

func (l *LinkedList[T]) RemoveLast() (T, bool) {
	if l.len == 0 || l.first == nil {
		var e T
		return e, false
	}
	if l.len == 1 {
		e := l.first.val
		l.first = nil
		l.len--
		return e, true
	}
	rn := l.first.prev
	rn.prev.next = rn.next
	rn.next.prev = rn.prev
	l.len--
	return rn.val, true
}

func (l *LinkedList[T]) PeekFirst() (T, bool) {
	if l.len == 0 || l.first == nil {
		var e T
		return e, false
	}
	return l.first.val, true
}

func (l *LinkedList[T]) PeekLast() (T, bool) {
	if l.len == 0 || l.first == nil {
		var e T
		return e, false
	}
	return l.first.prev.val, true
}

func (l *LinkedList[T]) ForEach(handler func(index int, e T)) {
	e := l.first
	for i := 0; i < l.len; i++ {
		handler(i, e.val)
	}
}
