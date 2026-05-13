package agent

import (
	"context"
	"errors"
	"reflect"
	"testing"
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

	queue.Flush(context.Background())

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

	queue.Flush(context.Background())

	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
	if queue.Len() != 0 {
		t.Fatalf("Len() = %d, want empty queue", queue.Len())
	}
}
