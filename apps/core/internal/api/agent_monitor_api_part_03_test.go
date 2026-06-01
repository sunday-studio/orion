package api

import (
	"net/http"
	"orion/core/internal/db"
	"testing"
	"time"
)

func TestListAllMonitorsFiltersCanonicalMonitorTypeAliases(t *testing.T) {
	server := setupTestServer(t)
	agent := db.Agent{ID: "agent-monitor-type-aliases", MachineId: "machine-monitor-type-aliases", Name: "monitor type aliases", OS: "linux", Arch: "arm64", Token: "token-monitor-type-aliases", LastSeen: time.Now()}
	if err := server.db.Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	monitors := []db.Monitor{{ID: "monitor-docker-container-type", AgentID: agent.ID, Name: "docker container type", Type: "docker-container", Lifecycle: "active", Health: "up"}, {ID: "monitor-systemd-service-type", AgentID: agent.ID, Name: "systemd service type", Type: "systemd-service", Lifecycle: "active", Health: "up"}}
	if err := server.db.Create(&monitors).Error; err != nil {
		t.Fatalf("create monitors: %v", err)
	}
	for _, tc := range []struct {
		query string
		want  string
	}{{query: "docker-container", want: "monitor-docker-container-type"}, {query: "docker", want: "monitor-docker-container-type"}, {query: "systemd-service", want: "monitor-systemd-service-type"}, {query: "systemd", want: "monitor-systemd-service-type"}} {
		resp := performJSONRequest(t, server, http.MethodGet, "/v1/monitors?type="+tc.query, nil, "")
		if resp.Code != http.StatusOK {
			t.Fatalf("type %q monitor list status = %d, body = %s", tc.query, resp.Code, resp.Body.String())
		}
		var listed struct {
			Success bool `json:"success"`
			Data    struct {
				Monitors []struct {
					ID   string `json:"id"`
					Type string `json:"type"`
				} `json:"monitors"`
				Count int64 `json:"count"`
			} `json:"data"`
		}
		decodeResponse(t, resp, &listed)
		if !listed.Success || listed.Data.Count != 1 || listed.Data.Monitors[0].ID != tc.want {
			t.Fatalf("type %q monitor list = %+v, want %s", tc.query, listed, tc.want)
		}
	}
}
func TestListAllMonitorsFiltersByOwnerAndSource(t *testing.T) {
	server := setupTestServer(t)
	agent := db.Agent{ID: "agent-monitor-owner-filter", MachineId: "machine-monitor-owner-filter", Name: "Customer App Agent", OS: "linux", Arch: "arm64", Token: "token-monitor-owner-filter", LastSeen: time.Now()}
	coreOwner := db.Agent{ID: "agent-core-monitor-owner-filter", MachineId: "core", Name: "Orion Core", OS: "linux", Arch: "arm64", Token: "token-core-monitor-owner-filter", LastSeen: time.Now()}
	if err := server.db.Create(&[]db.Agent{agent, coreOwner}).Error; err != nil {
		t.Fatalf("create owner filter agents: %v", err)
	}
	monitors := []db.Monitor{{ID: "monitor-agent-owner-filter", AgentID: agent.ID, Name: "Agent API check", Type: "http-healthcheck", Lifecycle: "active", Health: "up"}, {ID: "monitor-core-owner-filter", AgentID: coreOwner.ID, Name: "Core API check", Type: "http", Lifecycle: "active", Health: "up"}}
	if err := server.db.Create(&monitors).Error; err != nil {
		t.Fatalf("create owner filter monitors: %v", err)
	}
	if err := server.db.Create(&db.CoreMonitorConfig{MonitorID: "monitor-core-owner-filter", Kind: "http", ConfigJSON: "{}", SecretRefJSON: "{}", IntervalSeconds: 60, TimeoutSeconds: 10, NextRunAt: time.Now().Add(time.Minute)}).Error; err != nil {
		t.Fatalf("create core monitor config: %v", err)
	}
	for _, tc := range []struct {
		query     string
		wantID    string
		wantOwner string
		wantName  string
	}{{query: "owner_kind=agent", wantID: "monitor-agent-owner-filter", wantOwner: "agent", wantName: "Customer App Agent"}, {query: "owner_kind=core", wantID: "monitor-core-owner-filter", wantOwner: "core", wantName: "Orion Core"}, {query: "source=agent", wantID: "monitor-agent-owner-filter", wantOwner: "agent", wantName: "Customer App Agent"}, {query: "source=core", wantID: "monitor-core-owner-filter", wantOwner: "core", wantName: "Orion Core"}, {query: "owner_name=Customer", wantID: "monitor-agent-owner-filter", wantOwner: "agent", wantName: "Customer App Agent"}, {query: "owner_name=Orion", wantID: "monitor-core-owner-filter", wantOwner: "core", wantName: "Orion Core"}} {
		resp := performJSONRequest(t, server, http.MethodGet, "/v1/monitors?"+tc.query, nil, "")
		if resp.Code != http.StatusOK {
			t.Fatalf("%s monitor list status = %d, body = %s", tc.query, resp.Code, resp.Body.String())
		}
		var listed struct {
			Success bool `json:"success"`
			Data    struct {
				Monitors []struct {
					ID        string `json:"id"`
					OwnerKind string `json:"owner_kind"`
					OwnerName string `json:"owner_name"`
					Source    string `json:"source"`
				} `json:"monitors"`
				Count int64 `json:"count"`
			} `json:"data"`
		}
		decodeResponse(t, resp, &listed)
		if !listed.Success || listed.Data.Count != 1 || len(listed.Data.Monitors) != 1 {
			t.Fatalf("%s listed monitors = %+v, want exactly one monitor", tc.query, listed)
		}
		got := listed.Data.Monitors[0]
		if got.ID != tc.wantID || got.OwnerKind != tc.wantOwner || got.OwnerName != tc.wantName || got.Source != tc.wantOwner {
			t.Fatalf("%s monitor = %+v, want id %s owner/source %s name %s", tc.query, got, tc.wantID, tc.wantOwner, tc.wantName)
		}
	}
}
func TestListAllMonitorsFiltersByComputedHealth(t *testing.T) {
	server := setupTestServer(t)
	agent := db.Agent{ID: "agent-computed-monitor-filter", MachineId: "machine-computed-monitor-filter", Name: "computed monitor filter", OS: "linux", Arch: "arm64", Token: "token-computed-monitor-filter", LastSeen: time.Now()}
	if err := server.db.Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	monitor := db.Monitor{ID: "monitor-computed-degraded-filter", AgentID: agent.ID, Name: "computed degraded filter", Type: "http", Lifecycle: "active", Health: "up", ReportingIntervalSeconds: 60, CreatedAt: time.Now()}
	if err := server.db.Create(&monitor).Error; err != nil {
		t.Fatalf("create monitor: %v", err)
	}
	now := time.Now().UTC()
	reports := []db.MonitorReport{{ID: "report-computed-filter-1", MonitorID: monitor.ID, Payload: "{}", CollectedAt: now.Format(time.RFC3339), Health: "up", CreatedAt: now}, {ID: "report-computed-filter-2", MonitorID: monitor.ID, Payload: "{}", CollectedAt: now.Add(-1 * time.Minute).Format(time.RFC3339), Health: "up", CreatedAt: now.Add(-1 * time.Minute)}, {ID: "report-computed-filter-3", MonitorID: monitor.ID, Payload: "{}", CollectedAt: now.Add(-2 * time.Minute).Format(time.RFC3339), Health: "down", CreatedAt: now.Add(-2 * time.Minute)}, {ID: "report-computed-filter-4", MonitorID: monitor.ID, Payload: "{}", CollectedAt: now.Add(-3 * time.Minute).Format(time.RFC3339), Health: "down", CreatedAt: now.Add(-3 * time.Minute)}, {ID: "report-computed-filter-5", MonitorID: monitor.ID, Payload: "{}", CollectedAt: now.Add(-4 * time.Minute).Format(time.RFC3339), Health: "up", CreatedAt: now.Add(-4 * time.Minute)}}
	if err := server.db.Create(&reports).Error; err != nil {
		t.Fatalf("create reports: %v", err)
	}
	resp := performJSONRequest(t, server, http.MethodGet, "/v1/monitors?health=degraded", nil, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("computed degraded monitor list status = %d, body = %s", resp.Code, resp.Body.String())
	}
	var listed struct {
		Success bool `json:"success"`
		Data    struct {
			Monitors []struct {
				ID             string `json:"id"`
				Health         string `json:"health"`
				ComputedHealth string `json:"computed_health"`
			} `json:"monitors"`
			Count int64 `json:"count"`
		} `json:"data"`
	}
	decodeResponse(t, resp, &listed)
	if !listed.Success || listed.Data.Count != 1 || listed.Data.Monitors[0].ID != monitor.ID || listed.Data.Monitors[0].Health != "degraded" || listed.Data.Monitors[0].ComputedHealth != "degraded" {
		t.Fatalf("computed degraded monitor list = %+v, want computed degraded monitor", listed)
	}
}
