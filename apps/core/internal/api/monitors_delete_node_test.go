package api

import (
	"net/http"
	"orion/core/internal/db"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestDeleteRemovesNodeMonitorsAddedInOrionAndRefusesNodeConfigOnes(t *testing.T) {
	server := setupTestServer(t)
	node := registerTestNode(t, server)
	nodeID := node.Data.NodeID
	now := time.Now().UTC()
	for _, m := range []db.Monitor{
		{ID: "mon-added", NodeID: nodeID, Name: "disk /", Type: "disk", Lifecycle: "active", Health: "up", ComputedHealth: "up", IncidentState: "up", ReportingIntervalSeconds: 30, CreatedAt: now},
		{ID: "mon-bulk", NodeID: nodeID, Name: "memory", Type: "memory", Lifecycle: "active", Health: "up", ComputedHealth: "up", IncidentState: "up", ReportingIntervalSeconds: 30, CreatedAt: now},
		{ID: "mon-config", NodeID: nodeID, Name: "from node config", Type: "port", Lifecycle: "active", Health: "up", ComputedHealth: "up", IncidentState: "up", ReportingIntervalSeconds: 30, CreatedAt: now},
	} {
		if err := server.db.Create(&m).Error; err != nil {
			t.Fatal(err)
		}
	}
	for id, name := range map[string]string{"def-added": "disk /", "def-bulk": "memory"} {
		if err := server.db.Create(&db.NodeMonitorDefinition{ID: id, NodeID: nodeID, Name: name, CheckType: "disk", Params: "{}", IntervalSeconds: 30, FailAfter: 3, CreatedAt: now}).Error; err != nil {
			t.Fatal(err)
		}
	}
	lifecycle := func(id string) string {
		var m db.Monitor
		server.db.Where("id = ?", id).First(&m)
		return m.Lifecycle
	}
	definitions := func() int64 {
		var n int64
		server.db.Model(&db.NodeMonitorDefinition{}).Count(&n)
		return n
	}

	// Single delete (monitor detail page).
	if r := performJSONRequest(t, server, http.MethodDelete, "/v1/monitors/mon-added", nil, ""); r.Code != http.StatusOK {
		t.Fatalf("delete added monitor = %d %s", r.Code, r.Body.String())
	}
	if lifecycle("mon-added") != "deleted" || definitions() != 1 {
		t.Fatalf("lifecycle %q, %d definitions left; want deleted and 1", lifecycle("mon-added"), definitions())
	}

	// A monitor the node declares itself is refused with a reason, not a generic failure.
	if r := performJSONRequest(t, server, http.MethodDelete, "/v1/monitors/mon-config", nil, ""); r.Code != http.StatusBadRequest {
		t.Fatalf("delete node-config monitor = %d", r.Code)
	}
	if lifecycle("mon-config") != "active" {
		t.Fatal("a node-config monitor must stay")
	}

	// Bulk delete (monitors page): added one succeeds, node-config one fails with the reason.
	r := performJSONRequest(t, server, http.MethodPost, "/v1/monitors/bulk", gin.H{"ids": []string{"mon-bulk", "mon-config"}, "action": "delete"}, "")
	var bulk struct {
		Data struct {
			Results   []BulkMonitorResult `json:"results"`
			Succeeded int                 `json:"succeeded"`
		} `json:"data"`
	}
	decodeResponse(t, r, &bulk)
	if bulk.Data.Succeeded != 1 || !bulk.Data.Results[0].OK || bulk.Data.Results[1].OK || bulk.Data.Results[1].Error == "" {
		t.Fatalf("bulk delete = %+v", bulk.Data)
	}
	if lifecycle("mon-bulk") != "deleted" || definitions() != 0 {
		t.Fatalf("bulk: lifecycle %q, %d definitions", lifecycle("mon-bulk"), definitions())
	}
}
