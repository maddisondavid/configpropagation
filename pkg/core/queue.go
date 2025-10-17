package core

import (
	"sync"
)

// WorkQueue is a minimal FIFO queue with de-duplication.
type WorkQueue[T comparable] struct {
	mu    sync.Mutex
	set   map[T]struct{}
	items []T
}

func NewWorkQueue[T comparable]() *WorkQueue[T] {
	return &WorkQueue[T]{set: make(map[T]struct{}), items: make([]T, 0)}
}

func (q *WorkQueue[T]) Add(item T) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if _, exists := q.set[item]; exists {
		return
	}
	q.set[item] = struct{}{}
	q.items = append(q.items, item)
}

func (q *WorkQueue[T]) Get() (T, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	var zero T
	if len(q.items) == 0 {
		return zero, false
	}
	item := q.items[0]
	q.items = q.items[1:]
	delete(q.set, item)
	return item, true
}

func (q *WorkQueue[T]) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}
