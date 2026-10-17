package utils

import (
	"crypto/rand"
	"fmt"
)

// NewReportID returns a random UUIDv4 used as the idempotency key of a report, so
// that replays from the retry queue or spool are recognised by Core.
func NewReportID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(fmt.Sprintf("report id: %v", err))
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
