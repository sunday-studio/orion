package agent

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"orion/agent/internal/config"
	"orion/agent/internal/transport"
)

// ORB-016: token refresh racing with reads from reporting goroutines must be clean under -race.
func TestConcurrentStateRefreshAndReadsAreRaceFree(t *testing.T) {
	store := openAgentTestStore(t)
	if err := store.UpdateRegistration("agent-1", "token-0", "http://localhost:8999"); err != nil {
		t.Fatal(err)
	}
	internalState, _ := store.Get()
	a := New(&config.UserConfig{CoreURL: "http://localhost:8999"}, store, internalState)

	var wg sync.WaitGroup
	stop := make(chan struct{})
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					_ = a.agentID()
					_ = a.monitorsSnapshot()
					_ = a.isInMaintenanceMode()
				}
			}
		}()
	}
	for i := 0; i < 200; i++ {
		_ = store.ApplyReplacementToken("token-rotated")
		a.refreshInternalState()
		a.transport.SetAuthToken("token-x")
	}
	close(stop)
	wg.Wait()
	if a.agentID() != "agent-1" {
		t.Fatalf("agentID() = %q, want agent-1", a.agentID())
	}
}

// ORB-017: an early return must not drop queued reports.
func TestRetryQueueKeepsRemainingItemsWhenFlushIsCancelled(t *testing.T) {
	q := NewRetryQueue(10)
	var sent atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for i := 0; i < 5; i++ {
		i := i
		q.Push(RetryItem{Name: "item", Send: func(context.Context) error {
			sent.Add(1)
			if i == 1 {
				cancel() // shutdown arrives after the second item
			}
			return nil
		}})
	}
	if err := q.Flush(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Flush() error = %v, want context.Canceled", err)
	}
	if sent.Load() != 2 || q.Len() != 3 {
		t.Fatalf("sent = %d queued = %d, want 2 sent and the other 3 still queued", sent.Load(), q.Len())
	}
}

func TestRetryQueueKeepsAllUnsentItemsOnAuthError(t *testing.T) {
	q := NewRetryQueue(10)
	authErr := transport.NewAuthError(401, []byte(`{"message":"bad token"}`))
	for i := 0; i < 4; i++ {
		i := i
		q.Push(RetryItem{Name: "item", Send: func(context.Context) error {
			if i == 1 {
				return authErr
			}
			return nil
		}})
	}
	if err := q.Flush(context.Background()); !transport.IsAuthError(err) {
		t.Fatalf("Flush() error = %v, want auth error", err)
	}
	if q.Len() != 2 {
		t.Fatalf("queued after auth error = %d, want the 2 untouched items after the rejected one", q.Len())
	}
}

func TestRetryQueueRequeueRespectsCapacityAndOrder(t *testing.T) {
	q := NewRetryQueue(3)
	var order []string
	mk := func(name string, fail bool) RetryItem {
		return RetryItem{Name: name, Send: func(context.Context) error {
			order = append(order, name)
			if fail {
				return errors.New("down")
			}
			return nil
		}}
	}
	q.Push(mk("a", true))
	q.Push(mk("b", true))
	if err := q.Flush(context.Background()); err == nil {
		t.Fatal("expected flush error")
	}
	if q.Len() != 2 {
		t.Fatalf("len = %d, want failed items requeued", q.Len())
	}
	order = nil
	_ = q.Flush(context.Background())
	if len(order) != 2 || order[0] != "a" || order[1] != "b" {
		t.Fatalf("flush order = %v, want a then b", order)
	}
}

// ORB-018: a monitor that is not registered yet is retried, not silently abandoned.
func TestWaitForMonitorStateStopsOnCancelAndFindsMonitorWhenItAppears(t *testing.T) {
	store := openAgentTestStore(t)
	if err := store.UpdateRegistration("agent-1", "token", "http://localhost:8999"); err != nil {
		t.Fatal(err)
	}
	internalState, _ := store.Get()
	a := New(&config.UserConfig{CoreURL: "http://localhost:8999"}, store, internalState)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan *config.InternalStateMonitor, 1)
	go func() { done <- a.waitForMonitorState(ctx, "web") }()
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case got := <-done:
		if got != nil {
			t.Fatalf("waitForMonitorState() = %+v, want nil after cancel", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("waitForMonitorState did not return after context cancel")
	}

	if err := store.ReplaceMonitors([]config.InternalStateMonitor{{ID: "m1", Name: "web"}}); err != nil {
		t.Fatal(err)
	}
	if got := a.waitForMonitorState(context.Background(), "web"); got == nil || got.ID != "m1" {
		t.Fatalf("waitForMonitorState() = %+v, want the registered monitor", got)
	}
}

func TestIntervalOrDefaultFallsBackInsteadOfDisablingTheWorker(t *testing.T) {
	if got := intervalOrDefault("m", "30s"); got != 30*time.Second {
		t.Fatalf("valid interval = %v", got)
	}
	for _, bad := range []string{"", "banana", "-5s", "0s"} {
		if got := intervalOrDefault("m", bad); got != defaultWorkerInterval {
			t.Fatalf("intervalOrDefault(%q) = %v, want default %v", bad, got, defaultWorkerInterval)
		}
	}
}
