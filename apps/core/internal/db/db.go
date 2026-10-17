package db

import (
	"cmp"
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"fmt"
	"orion/core/internal/utils"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

//go:embed migrations/*.up.sql
var migrationFiles embed.FS

func Initialize(dataDir string, logLevel string) (*gorm.DB, error) {
	return OpenFile(dataDir, "orion.db", logLevel)
}

// OpenFile opens (creating if needed) a SQLite database named fileName inside
// dataDir with the production profile: WAL, busy timeout, bounded pool, UTC
// timestamps and owner-only permissions. Core uses it for orion.db and logs.db.
func OpenFile(dataDir string, fileName string, logLevel string) (*gorm.DB, error) {
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return nil, err
	}

	dbPath := filepath.Join(dataDir, fileName)

	db, err := gorm.Open(sqlite.Open(sqliteDSN(dbPath)), &gorm.Config{
		Logger:  newGormLogger(logLevel),
		NowFunc: func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxIdleConns(sqliteMaxIdleConns)
	sqlDB.SetMaxOpenConns(sqliteMaxOpenConns)

	// The database holds secrets or user data; keep it owner-only.
	if err := os.Chmod(dbPath, 0600); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	return db, nil
}

// Migrate runs explicit SQL migrations embedded in the Core binary.
func Migrate(db *gorm.DB) error {
	log := utils.NewLogger()
	return MigrateWithFiles(db, "migrations", log)
}

// MigrateWithFiles runs embedded SQL migration files and records applied versions.
//
// Safety properties:
//   - Single migrator: each migration runs inside BEGIN IMMEDIATE and re-checks
//     that it is still pending, so API and Worker starting together apply it once.
//   - Tamper detection: every applied migration stores a SHA-256 of its file;
//     editing an applied file is reported as an error at the next start.
//   - Backup: before applying anything to a database that already has migrations,
//     a consistent copy is written next to it (see backupBeforeMigration).
func MigrateWithFiles(db *gorm.DB, migrationsPath string, logger *utils.Logger) error {
	logger.Info("Running database migrations")

	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	ctx := context.Background()

	if _, err := sqlDB.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, name TEXT NOT NULL, applied_at DATETIME NOT NULL)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	if err := ensureMigrationChecksumColumn(sqlDB); err != nil {
		return err
	}

	migrations, err := loadMigrations(migrationsPath)
	if err != nil {
		return err
	}
	migrations = append(migrations, goMigrations()...)
	slices.SortFunc(migrations, func(a, b migration) int { return cmp.Compare(a.version, b.version) })

	applied, err := appliedMigrationChecksums(sqlDB)
	if err != nil {
		return err
	}
	var pending []migration
	for _, m := range migrations {
		stored, done := applied[m.version]
		if !done {
			pending = append(pending, m)
			continue
		}
		switch {
		case stored == "":
			// Applied before checksums existed: trust it once and record the current hash.
			if _, err := sqlDB.Exec(`UPDATE schema_migrations SET checksum = ? WHERE version = ? AND checksum = ''`, m.checksum(), m.version); err != nil {
				return fmt.Errorf("record checksum for migration %s: %w", m.name, err)
			}
		case stored != m.checksum():
			return fmt.Errorf("migration %s was modified after it was applied (checksum mismatch); restore the original file and add a new migration instead", m.name)
		}
	}

	if len(pending) > 0 && len(applied) > 0 {
		if err := backupBeforeMigration(sqlDB, pending[0].version, logger); err != nil {
			return err
		}
	}

	for _, m := range pending {
		ran, err := applyMigration(ctx, sqlDB, m)
		if err != nil {
			logger.Error("Migration failed", "version", m.version, "name", m.name, "error", err)
			return err
		}
		if ran {
			logger.Info("Migration applied", "version", m.version, "name", m.name)
		}
	}

	logger.Info("Database migrations completed successfully")
	return nil
}

type migration struct {
	version int
	name    string
	sql     string
	// apply, when set, replaces sql for migrations that must inspect the schema.
	apply func(ctx context.Context, conn *sql.Conn) error
}

func (m migration) checksum() string {
	content := strings.ReplaceAll(m.sql, "\r\n", "\n")
	if m.apply != nil {
		content = "go:" + m.name
	}
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

// goMigrations are numbered migrations written in Go because plain SQL cannot
// express "add this column only if it is missing".
func goMigrations() []migration {
	return []migration{{
		version: 32,
		name:    "000032_agent_report_metadata_columns.go",
		apply:   ensureAgentReportMetadataColumns,
	}}
}

func ensureMigrationChecksumColumn(db *sql.DB) error {
	columns, err := tableColumns(context.Background(), db, "schema_migrations")
	if err != nil {
		return err
	}
	if columns["checksum"] {
		return nil
	}
	if _, err := db.Exec(`ALTER TABLE schema_migrations ADD COLUMN checksum TEXT NOT NULL DEFAULT ''`); err != nil {
		// A parallel process may have added it between the check and the ALTER.
		if again, checkErr := tableColumns(context.Background(), db, "schema_migrations"); checkErr == nil && again["checksum"] {
			return nil
		}
		return fmt.Errorf("add schema_migrations.checksum: %w", err)
	}
	return nil
}

func appliedMigrationChecksums(db *sql.DB) (map[int]string, error) {
	rows, err := db.Query(`SELECT version, checksum FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("read applied migrations: %w", err)
	}
	defer rows.Close()
	applied := map[int]string{}
	for rows.Next() {
		var version int
		var checksum string
		if err := rows.Scan(&version, &checksum); err != nil {
			return nil, err
		}
		applied[version] = checksum
	}
	return applied, rows.Err()
}

func loadMigrations(path string) ([]migration, error) {
	entries, err := migrationFiles.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}

	migrations := make([]migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".up.sql") {
			continue
		}

		versionText, _, ok := strings.Cut(entry.Name(), "_")
		if !ok {
			return nil, fmt.Errorf("invalid migration file name: %s", entry.Name())
		}
		version, err := strconv.Atoi(versionText)
		if err != nil {
			return nil, fmt.Errorf("invalid migration version %q: %w", versionText, err)
		}

		content, err := migrationFiles.ReadFile(filepath.Join(path, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}

		migrations = append(migrations, migration{
			version: version,
			name:    entry.Name(),
			sql:     string(content),
		})
	}

	slices.SortFunc(migrations, func(a, b migration) int {
		return cmp.Compare(a.version, b.version)
	})

	return migrations, nil
}

// applyMigration runs one migration under an immediate (write-locked)
// transaction on a dedicated connection. It re-checks the version inside the lock
// so a concurrent migrator that got there first makes this a no-op (ran=false).
func applyMigration(ctx context.Context, db *sql.DB, m migration) (ran bool, err error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return false, err
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return false, fmt.Errorf("lock database for migration %s: %w", m.name, err)
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(ctx, "ROLLBACK")
		}
	}()

	var already int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, m.version).Scan(&already); err != nil {
		return false, fmt.Errorf("check migration version %d: %w", m.version, err)
	}
	if already > 0 {
		return false, nil
	}

	if m.apply != nil {
		if err := m.apply(ctx, conn); err != nil {
			return false, fmt.Errorf("apply migration %s: %w", m.name, err)
		}
	} else if _, err := conn.ExecContext(ctx, m.sql); err != nil {
		return false, fmt.Errorf("apply migration %s: %w", m.name, err)
	}
	if _, err := conn.ExecContext(ctx, `INSERT INTO schema_migrations (version, name, applied_at, checksum) VALUES (?, ?, ?, ?)`, m.version, m.name, time.Now().UTC(), m.checksum()); err != nil {
		return false, fmt.Errorf("record migration %s: %w", m.name, err)
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return false, fmt.Errorf("commit migration %s: %w", m.name, err)
	}
	committed = true
	return true, nil
}

func ensureAgentReportMetadataColumns(ctx context.Context, conn *sql.Conn) error {
	columns, err := tableColumns(ctx, conn, "agent_reports")
	if err != nil {
		return err
	}
	if len(columns) == 0 {
		return nil
	}

	for _, column := range []struct {
		name string
		sql  string
	}{
		{name: "agent_version", sql: `ALTER TABLE agent_reports ADD COLUMN agent_version TEXT`},
		{name: "config_summary", sql: `ALTER TABLE agent_reports ADD COLUMN config_summary TEXT`},
	} {
		if columns[column.name] {
			continue
		}
		if _, err := conn.ExecContext(ctx, column.sql); err != nil {
			return fmt.Errorf("add agent_reports.%s column: %w", column.name, err)
		}
	}

	return nil
}

type columnQuerier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func tableColumns(ctx context.Context, db columnQuerier, table string) (map[string]bool, error) {
	rows, err := db.QueryContext(ctx, fmt.Sprintf(`PRAGMA table_info(%s)`, table))
	if err != nil {
		return nil, fmt.Errorf("inspect %s columns: %w", table, err)
	}
	defer rows.Close()

	columns := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull int
		var defaultValue any
		var primaryKey int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return nil, fmt.Errorf("scan %s column: %w", table, err)
		}
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate %s columns: %w", table, err)
	}

	return columns, nil
}
