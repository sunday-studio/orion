package state

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/glebarez/go-sqlite"
)

func writeLegacyState(t *testing.T, path string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, stmt := range []string{
		`CREATE TABLE agent_state (id INTEGER PRIMARY KEY, agent_id TEXT, token TEXT, registered BOOLEAN NOT NULL DEFAULT false, core_url TEXT, last_sync DATETIME, maintenance_mode BOOLEAN NOT NULL DEFAULT false, maintenance_reason TEXT, maintenance_until DATETIME, created_at DATETIME, updated_at DATETIME)`,
		`INSERT INTO agent_state (id, agent_id, token, registered, core_url) VALUES (1, 'agent-1', 'tok', 1, 'https://core.example.com')`,
		`CREATE TABLE monitor_state (name TEXT PRIMARY KEY, monitor_id TEXT NOT NULL, status TEXT NOT NULL, last_checked DATETIME, created_at DATETIME, updated_at DATETIME)`,
		`CREATE TABLE report_spool (id INTEGER PRIMARY KEY, kind TEXT NOT NULL, agent_id TEXT NOT NULL, monitor_id TEXT, monitor_name TEXT, payload_json TEXT NOT NULL, attempts INTEGER NOT NULL DEFAULT 0, last_error TEXT, next_attempt_at DATETIME, created_at DATETIME)`,
		`INSERT INTO report_spool (kind, agent_id, payload_json) VALUES ('system', 'agent-1', '{}')`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("%v\n%s", err, stmt)
		}
	}
}

func TestAStateKeptUnderTheOldNamesKeepsItsRegistration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	writeLegacyState(t, path)

	// A status check before the daemon has run still sees the registration.
	st, err := InspectReadOnly(path)
	if err != nil || !st.IsRegistered() || st.NodeID != "agent-1" || st.Token != "tok" {
		t.Fatalf("read-only view of an old state: %+v %v", st, err)
	}

	// Opening it renames the table and column in place; nothing is lost.
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	got, err := store.Get()
	if err != nil || !got.IsRegistered() || got.NodeID != "agent-1" || got.Token != "tok" || got.CoreURL != "https://core.example.com" {
		t.Fatalf("after Open: %+v %v", got, err)
	}
	var spooled int64
	if err := store.db.Table("report_spool").Where("node_id = ?", "agent-1").Count(&spooled).Error; err != nil || spooled != 1 {
		t.Fatalf("spool rows after the rename = %d (%v)", spooled, err)
	}
	if store.db.Migrator().HasTable("agent_state") {
		t.Fatal("agent_state should have been renamed")
	}
}
