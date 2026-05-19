package agent

import (
	"context"
	"sync"

	"orion/agent/internal/logging"
	"orion/agent/internal/transport"
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
		logging.Debugf("retry queue full; dropped oldest item and queued %s (len=%d capacity=%d)", item.Name, len(q.items), q.capacity)
		return
	}
	q.items = append(q.items, item)
	logging.Debugf("retry item queued: name=%s len=%d capacity=%d", item.Name, len(q.items), q.capacity)
}

func (q *RetryQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

func (q *RetryQueue) Flush(ctx context.Context) error {
	q.mu.Lock()
	items := q.items
	q.items = nil
	q.mu.Unlock()
	logging.Debugf("retry queue flush started: items=%d", len(items))

	var firstErr error
	for _, item := range items {
		select {
		case <-ctx.Done():
			q.Push(item)
			return ctx.Err()
		default:
		}

		if err := item.Send(ctx); err != nil {
			if transport.IsAuthError(err) {
				logging.Debugf("retry item failed with auth error: name=%s", item.Name)
				return err
			}
			q.Push(item)
			logging.Debugf("retry item failed and was requeued: name=%s error=%v", item.Name, err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		logging.Debugf("retry item sent successfully: name=%s", item.Name)
	}
	return firstErr
}
