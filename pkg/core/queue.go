package core

import (
	"sync"
)

// WorkQueue is a minimal FIFO queue with de-duplication.
type WorkQueue[T comparable] struct {
	mutex sync.Mutex
	set   map[T]struct{}
	items []T
}

// NewWorkQueue constructs an empty WorkQueue.
func NewWorkQueue[T comparable]() *WorkQueue[T] {
	return &WorkQueue[T]{set: make(map[T]struct{}), items: make([]T, 0)}
}

// Add enqueues an item if it is not already present.
func (queue *WorkQueue[T]) Add(item T) {
	queue.mutex.Lock()
	defer queue.mutex.Unlock()

	if _, exists := queue.set[item]; exists {
		return
	}

	queue.set[item] = struct{}{}
	queue.items = append(queue.items, item)
}

// Get dequeues the next item, returning false when the queue is empty.
func (queue *WorkQueue[T]) Get() (T, bool) {
	queue.mutex.Lock()
	defer queue.mutex.Unlock()

	var zero T

	if len(queue.items) == 0 {
		return zero, false
	}

	item := queue.items[0]

	queue.items = queue.items[1:]
	delete(queue.set, item)

	return item, true
}

// Len returns the number of queued items.
func (queue *WorkQueue[T]) Len() int {
	queue.mutex.Lock()
	defer queue.mutex.Unlock()

	return len(queue.items)
}
