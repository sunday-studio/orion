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

// Flush sends every queued item. Nothing is ever dropped by an early return:
// when the context is cancelled, the item in hand and everything not yet
// attempted go back; when Core rejects credentials, the rejected item is dropped
// and everything not yet attempted goes back on the queue (ahead of anything pushed
// meanwhile) for the next flush or a restart-time flush.
func (q *RetryQueue) Flush(ctx context.Context) error {
	q.mu.Lock()
	items := q.items
	q.items = nil
	q.mu.Unlock()
	logging.Debugf("retry queue flush started: items=%d", len(items))

	var firstErr error
	for i, item := range items {
		select {
		case <-ctx.Done():
			q.requeueFront(items[i:])
			return ctx.Err()
		default:
		}

		if err := item.Send(ctx); err != nil {
			if transport.IsAuthError(err) {
				logging.Debugf("retry item failed with auth error: name=%s", item.Name)
				// The rejected item is dropped (as before); the untouched rest wait for a replacement token.
				q.requeueFront(items[i+1:])
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

// requeueFront puts items back before anything queued since the flush began,
// trimming the oldest if capacity would be exceeded.
func (q *RetryQueue) requeueFront(items []RetryItem) {
	q.mu.Lock()
	defer q.mu.Unlock()
	merged := append(append([]RetryItem(nil), items...), q.items...)
	if len(merged) > q.capacity {
		merged = merged[len(merged)-q.capacity:]
	}
	q.items = merged
}
