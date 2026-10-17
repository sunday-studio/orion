package api

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"
)

// Review P2: an open live tail must not stall shutdown or turn it into a failure.
func TestServerShutsDownPromptlyWithAnOpenLiveTail(t *testing.T) {
	server := setupLogsServer(t, nil)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	listener.Close()

	ctx, stop := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Start(ctx, addr) }()

	var resp *http.Response
	for i := 0; i < 50; i++ {
		resp, err = http.Get(fmt.Sprintf("http://%s/v1/logs/stream", addr))
		if err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("could not open the stream: %v", err)
	}
	defer resp.Body.Close()

	started := time.Now()
	stop()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Start() returned %v on shutdown, want nil", err)
		}
		if elapsed := time.Since(started); elapsed > 3*time.Second {
			t.Fatalf("shutdown took %v with an open stream, want it to end the stream promptly", elapsed)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("Start() did not return: the open stream blocked shutdown")
	}
}
