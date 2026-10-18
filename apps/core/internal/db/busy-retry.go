package db

import (
	"strings"
	"time"
)

// SQLite answers some statements with SQLITE_BUSY immediately, without consulting the busy
// timeout: switching to WAL while another connection is mid-transaction, or the first
// schema statements on a brand-new file when several processes start together. Those are
// transient, so setup work retries them for a few seconds.
const busyRetryWindow = 10 * time.Second

func isSQLiteBusy(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "database is locked") || strings.Contains(msg, "SQLITE_BUSY") || strings.Contains(msg, "database table is locked")
}

func retryBusy(fn func() error) error {
	deadline := time.Now().Add(busyRetryWindow)
	delay := 10 * time.Millisecond
	for {
		err := fn()
		if !isSQLiteBusy(err) || time.Now().After(deadline) {
			return err
		}
		time.Sleep(delay)
		if delay < 200*time.Millisecond {
			delay *= 2
		}
	}
}
