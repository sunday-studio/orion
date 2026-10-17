package db

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"gorm.io/gorm"
)

type profileRow struct {
	ID    int `gorm:"primaryKey;autoIncrement"`
	Value string
}

func TestInitializeAppliesSQLiteProductionProfile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "data")
	database, err := Initialize(dir, "warn")
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	var journalMode string
	database.Raw("PRAGMA journal_mode").Scan(&journalMode)
	if !strings.EqualFold(journalMode, "wal") {
		t.Fatalf("journal_mode = %q, want wal", journalMode)
	}
	var busyTimeout int
	database.Raw("PRAGMA busy_timeout").Scan(&busyTimeout)
	if busyTimeout != sqliteBusyTimeoutMillis {
		t.Fatalf("busy_timeout = %d, want %d", busyTimeout, sqliteBusyTimeoutMillis)
	}
	sqlDB, _ := database.DB()
	if got := sqlDB.Stats().MaxOpenConnections; got != sqliteMaxOpenConns {
		t.Fatalf("MaxOpenConnections = %d, want %d", got, sqliteMaxOpenConns)
	}

	info, err := os.Stat(filepath.Join(dir, "orion.db"))
	if err != nil {
		t.Fatalf("stat db: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("db file mode = %v, want 0600", info.Mode().Perm())
	}
	dirInfo, _ := os.Stat(dir)
	if dirInfo.Mode().Perm() != 0700 {
		t.Fatalf("data dir mode = %v, want 0700", dirInfo.Mode().Perm())
	}
}

func TestInitializeUsesUTCForTimestamps(t *testing.T) {
	database, err := Initialize(t.TempDir(), "warn")
	if err != nil {
		t.Fatal(err)
	}
	if _, offset := database.NowFunc().Zone(); offset != 0 {
		t.Fatalf("NowFunc zone offset = %d, want UTC", offset)
	}
}

func TestSQLiteProfileSurvivesConcurrentWriters(t *testing.T) {
	database, err := Initialize(t.TempDir(), "silent")
	if err != nil {
		t.Fatal(err)
	}
	if err := database.AutoMigrate(&profileRow{}); err != nil {
		t.Fatal(err)
	}

	const writers, perWriter = 24, 25
	var wg sync.WaitGroup
	errs := make(chan error, writers*perWriter)
	for w := range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range perWriter {
				err := database.Transaction(func(tx *gorm.DB) error {
					var n int64
					if err := tx.Model(&profileRow{}).Count(&n).Error; err != nil {
						return err
					}
					return tx.Create(&profileRow{Value: fmt.Sprintf("w%d-%d", w, i)}).Error
				})
				if err != nil {
					errs <- err
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent write failed (database is locked?): %v", err)
	}
	var count int64
	database.Model(&profileRow{}).Count(&count)
	if count != writers*perWriter {
		t.Fatalf("rows = %d, want %d", count, writers*perWriter)
	}
}

type captureWriter struct{ lines []string }

func (c *captureWriter) Printf(format string, args ...any) {
	c.lines = append(c.lines, fmt.Sprintf(format, args...))
}

func TestGormLoggerDoesNotLogBoundValues(t *testing.T) {
	for _, level := range []string{"warn", "info"} {
		t.Run(level, func(t *testing.T) {
			w := &captureWriter{}
			database, err := Initialize(t.TempDir(), "silent")
			if err != nil {
				t.Fatal(err)
			}
			database = database.Session(&gorm.Session{Logger: newGormLoggerWithWriter(w, level)})
			if err := database.AutoMigrate(&profileRow{}); err != nil {
				t.Fatal(err)
			}
			const secret = "whsec_super_secret_value"
			if err := database.Create(&profileRow{Value: secret}).Error; err != nil {
				t.Fatal(err)
			}
			if got := strings.Join(w.lines, "\n"); strings.Contains(got, secret) {
				t.Fatalf("log output leaked bound value at level %s:\n%s", level, got)
			}
			if level == "warn" && len(w.lines) != 0 {
				t.Fatalf("warn level logged routine statements: %v", w.lines)
			}
		})
	}
}

func TestParseGormLogLevelDefaultsToWarn(t *testing.T) {
	if parseGormLogLevel("") != parseGormLogLevel("warn") || parseGormLogLevel("bogus") != parseGormLogLevel("warn") {
		t.Fatal("unknown or empty level must default to warn")
	}
}
