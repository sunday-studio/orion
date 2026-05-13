package db

import (
	"database/sql"
	"embed"
	"fmt"
	"orion/core/internal/logging"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

//go:embed migrations/*.up.sql
var migrationFiles embed.FS

func Initialize(dataDir string) (*gorm.DB, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}

	dbPath := filepath.Join(dataDir, "orion.db")

	gormLogger := logger.Default.LogMode(logger.Info)

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)

	return db, nil
}

// Migrate runs explicit SQL migrations embedded in the Core binary.
func Migrate(db *gorm.DB) error {
	log := logging.NewLogger()
	return MigrateWithFiles(db, "migrations", log)
}

// MigrateWithFiles runs embedded SQL migration files and records applied versions.
func MigrateWithFiles(db *gorm.DB, migrationsPath string, logger *logging.Logger) error {
	logger.Info("Running database migrations")

	sqlDB, err := db.DB()
	if err != nil {
		return err
	}

	if _, err := sqlDB.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, name TEXT NOT NULL, applied_at DATETIME NOT NULL)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	migrations, err := loadMigrations(migrationsPath)
	if err != nil {
		return err
	}

	for _, migration := range migrations {
		applied, err := migrationApplied(sqlDB, migration.version)
		if err != nil {
			return err
		}
		if applied {
			continue
		}
		if err := applyMigration(sqlDB, migration); err != nil {
			logger.Error("Migration failed", "version", migration.version, "name", migration.name, "error", err)
			return err
		}
		logger.Info("Migration applied", "version", migration.version, "name", migration.name)
	}

	if err := ensureAgentReportMetadataColumns(sqlDB); err != nil {
		return err
	}

	logger.Info("Database migrations completed successfully")
	return nil
}

type migration struct {
	version int
	name    string
	sql     string
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

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].version < migrations[j].version
	})

	return migrations, nil
}

func migrationApplied(db *sql.DB, version int) (bool, error) {
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, version).Scan(&count); err != nil {
		return false, fmt.Errorf("check migration version %d: %w", version, err)
	}
	return count > 0, nil
}

func applyMigration(db *sql.DB, migration migration) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(migration.sql); err != nil {
		return fmt.Errorf("apply migration %s: %w", migration.name, err)
	}
	if _, err := tx.Exec(`INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)`, migration.version, migration.name, time.Now().UTC()); err != nil {
		return fmt.Errorf("record migration %s: %w", migration.name, err)
	}
	return tx.Commit()
}

func ensureAgentReportMetadataColumns(db *sql.DB) error {
	columns, err := tableColumns(db, "agent_reports")
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
		if _, err := db.Exec(column.sql); err != nil {
			return fmt.Errorf("add agent_reports.%s column: %w", column.name, err)
		}
	}

	return nil
}

func tableColumns(db *sql.DB, table string) (map[string]bool, error) {
	rows, err := db.Query(fmt.Sprintf(`PRAGMA table_info(%s)`, table))
	if err != nil {
		return nil, fmt.Errorf("inspect %s columns: %w", table, err)
	}
	defer rows.Close()

	columns := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull int
		var defaultValue interface{}
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
