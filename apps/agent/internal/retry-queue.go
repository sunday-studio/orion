package agent

import (
	"context"
	"sync"
)

type RetryQueue struct {
	mu       sync.Mutex
	capacity int
	items    []RetryItem
}

type RetryItem struct {
	Name string
	Send func(context.Context) error
}

func NewRetryQueue(capacity int) *RetryQueue {
	if capacity <= 0 {
		capacity = 100
	}
	return &RetryQueue{capacity: capacity}
}

func (q *RetryQueue) Push(item RetryItem) {
	if item.Send == nil {
		return
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.items) >= q.capacity {
		copy(q.items, q.items[1:])
		q.items[len(q.items)-1] = item
		return
	}
	q.items = append(q.items, item)
}

func (q *RetryQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

func (q *RetryQueue) Flush(ctx context.Context) {
	q.mu.Lock()
	items := q.items
	q.items = nil
	q.mu.Unlock()

	for _, item := range items {
		select {
		case <-ctx.Done():
			q.Push(item)
			return
		default:
		}

		if err := item.Send(ctx); err != nil {
			q.Push(item)
		}
	}
}
