package db

import (
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"gorm.io/gorm/logger"
)

const (
	sqliteBusyTimeoutMillis = 5000
	sqliteMaxOpenConns      = 8
	sqliteMaxIdleConns      = 4
	slowQueryThreshold      = 200 * time.Millisecond
)

// sqliteDSN builds the connection string for a two-process (API + Worker),
// many-writer workload: WAL lets readers run beside the single writer,
// busy_timeout waits for the write lock instead of failing with SQLITE_BUSY, and
// an immediate transaction lock avoids read-to-write upgrade deadlocks.
func sqliteDSN(dbPath string) string {
	params := url.Values{}
	params.Set("_journal_mode", "WAL")
	params.Set("_busy_timeout", "5000")
	params.Set("_synchronous", "NORMAL")
	params.Set("_txlock", "immediate")
	params.Set("_foreign_keys", "on")
	// Only affects newly created databases; see docs/sqlite-backup-restore.md to convert an existing one.
	params.Set("_auto_vacuum", "incremental")
	return "file:" + dbPath + "?" + params.Encode()
}

// newGormLogger returns a logger that never prints bound SQL values. At the
// default level only slow queries and errors are logged, so secrets (webhook
// signing keys, token hashes, subscriber emails, report payloads) stay out of logs.
func newGormLogger(level string) logger.Interface {
	return newGormLoggerWithWriter(log.New(os.Stdout, "\r\n", log.LstdFlags), level)
}

func newGormLoggerWithWriter(w logger.Writer, level string) logger.Interface {
	cfg := logger.Config{
		SlowThreshold:             slowQueryThreshold,
		LogLevel:                  parseGormLogLevel(level),
		IgnoreRecordNotFoundError: true,
		ParameterizedQueries:      true,
	}
	return logger.New(w, cfg)
}

func parseGormLogLevel(level string) logger.LogLevel {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "silent":
		return logger.Silent
	case "error":
		return logger.Error
	case "info":
		return logger.Info
	default:
		return logger.Warn
	}
}
