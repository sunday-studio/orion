package node

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"orion/node/internal/transport"
)

func TestRetryQueueDropsOldestWhenFull(t *testing.T) {
	queue := NewRetryQueue(2)

	queue.Push(RetryItem{Name: "first", Send: func(context.Context) error { return nil }})
	queue.Push(RetryItem{Name: "second", Send: func(context.Context) error { return nil }})
	queue.Push(RetryItem{Name: "third", Send: func(context.Context) error { return nil }})

	if queue.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", queue.Len())
	}

	queue.mu.Lock()
	names := []string{queue.items[0].Name, queue.items[1].Name}
	queue.mu.Unlock()

	if !reflect.DeepEqual(names, []string{"second", "third"}) {
		t.Fatalf("queue names = %#v, want [second third]", names)
	}
}

func TestRetryQueueFlushRequeuesFailures(t *testing.T) {
	queue := NewRetryQueue(10)
	attempts := 0

	queue.Push(RetryItem{Name: "report", Send: func(context.Context) error {
		attempts++
		return errors.New("core unavailable")
	}})

	if err := queue.Flush(context.Background()); err == nil {
		t.Fatal("Flush() error = nil, want failed send error")
	}

	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
	if queue.Len() != 1 {
		t.Fatalf("Len() = %d, want failed item requeued", queue.Len())
	}
}

func TestRetryQueueFlushRemovesSuccesses(t *testing.T) {
	queue := NewRetryQueue(10)
	attempts := 0

	queue.Push(RetryItem{Name: "report", Send: func(context.Context) error {
		attempts++
		return nil
	}})

	if err := queue.Flush(context.Background()); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}

	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
	if queue.Len() != 0 {
		t.Fatalf("Len() = %d, want empty queue", queue.Len())
	}
}

func TestRetryQueueFlushReturnsAuthErrorWithoutRequeue(t *testing.T) {
	queue := NewRetryQueue(10)
	attempts := 0

	queue.Push(RetryItem{Name: "report", Send: func(context.Context) error {
		attempts++
		return &transport.AuthError{StatusCode: 401}
	}})

	err := queue.Flush(context.Background())
	if err == nil {
		t.Fatal("Flush() error = nil, want auth error")
	}
	if !transport.IsAuthError(err) {
		t.Fatalf("Flush() error = %T %[1]v, want auth error", err)
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
	if queue.Len() != 0 {
		t.Fatalf("Len() = %d, want auth-failed item dropped", queue.Len())
	}
}

func TestRetryQueueFlushKeepsRemainderOnAuthErrorAndCancel(t *testing.T) {
	mk := func(names ...string) *RetryQueue {
		q := NewRetryQueue(10)
		for i, n := range names {
			i := i
			q.Push(RetryItem{Name: n, Send: func(context.Context) error {
				if i == 0 {
					return &transport.AuthError{StatusCode: 401}
				}
				return nil
			}})
		}
		return q
	}
	q := mk("a", "b", "c")
	if err := q.Flush(context.Background()); !transport.IsAuthError(err) {
		t.Fatalf("err = %v", err)
	}
	if q.Len() != 2 {
		t.Fatalf("after auth error Len() = %d, want the 2 unsent items kept", q.Len())
	}

	q = mk("b", "c", "d")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := q.Flush(ctx); err == nil {
		t.Fatal("expected context error")
	}
	if q.Len() != 3 {
		t.Fatalf("after cancel Len() = %d, want all 3 kept", q.Len())
	}
}
