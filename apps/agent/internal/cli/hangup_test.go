package cli

import (
	"os"
	"syscall"
	"testing"
	"time"
)

// ORB-014: SIGHUP must not terminate the agent (Go's default would).
func TestSIGHUPDoesNotKillTheProcess(t *testing.T) {
	ignoreHangup()
	if err := syscall.Kill(os.Getpid(), syscall.SIGHUP); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	// Reaching this line means the process survived the signal.
}
