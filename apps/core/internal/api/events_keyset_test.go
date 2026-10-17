package api

import (
	"fmt"
	"net/http"
	"orion/core/internal/db"
	"testing"
	"time"
)

type eventsPage struct {
	Data struct {
		Events []struct {
			ID        string    `json:"id"`
			CreatedAt time.Time `json:"created_at"`
		} `json:"events"`
		NextCursor string `json:"next_cursor"`
		HasMore    bool   `json:"has_more"`
	} `json:"data"`
}

func TestEventsKeysetPaginationWalksEveryEventOnceInOrder(t *testing.T) {
	server := setupTestServer(t)
	base := time.Now().UTC().Add(-time.Hour)
	const rows = 120
	for i := 0; i < rows; i++ {
		// Some rows share a timestamp so the id tiebreaker is exercised across sources.
		at := base.Add(time.Duration(i/3) * time.Second)
		if err := server.db.Create(&db.IncidentEvent{ID: fmt.Sprintf("ie-%04d", i), IncidentID: "inc", Type: "note", Message: "m", CreatedAt: at}).Error; err != nil {
			t.Fatal(err)
		}
		if i%2 == 0 {
			if err := server.db.Create(&db.AlertDelivery{ID: fmt.Sprintf("ad-%04d", i), IncidentID: "inc", EventType: "incident_opened", Channel: "c", Type: "webhook", Status: "sent", CreatedAt: at}).Error; err != nil {
				t.Fatal(err)
			}
		}
	}
	seen := map[string]bool{}
	var prev time.Time
	cursor, pages := "", 0
	for {
		path := "/v1/events?limit=17"
		if cursor != "" {
			path += "&cursor=" + cursor
		}
		resp := performJSONRequest(t, server, http.MethodGet, path, nil, "")
		if resp.Code != http.StatusOK {
			t.Fatalf("status %d: %s", resp.Code, resp.Body.String())
		}
		var page eventsPage
		decodeResponse(t, resp, &page)
		for _, e := range page.Data.Events {
			if seen[e.ID] {
				t.Fatalf("event %s returned twice", e.ID)
			}
			seen[e.ID] = true
			if !prev.IsZero() && e.CreatedAt.After(prev) {
				t.Fatalf("order violated: %s after %s", e.CreatedAt, prev)
			}
			prev = e.CreatedAt
		}
		pages++
		if !page.Data.HasMore {
			break
		}
		cursor = page.Data.NextCursor
		if cursor == "" || pages > 50 {
			t.Fatalf("bad paging: cursor=%q pages=%d", cursor, pages)
		}
	}
	if want := rows + rows/2; len(seen) != want {
		t.Fatalf("saw %d events, want %d", len(seen), want)
	}
	if resp := performJSONRequest(t, server, http.MethodGet, "/v1/events?cursor=!!!", nil, ""); resp.Code != http.StatusBadRequest {
		t.Fatalf("bad cursor status = %d", resp.Code)
	}
	if resp := performJSONRequest(t, server, http.MethodGet, "/v1/events?offset=100000", nil, ""); resp.Code != http.StatusBadRequest {
		t.Fatalf("huge offset status = %d", resp.Code)
	}
	// Filtered paging still fills pages from a source with sparse matches.
	resp := performJSONRequest(t, server, http.MethodGet, "/v1/events?source=alert_delivery&limit=10", nil, "")
	var filtered eventsPage
	decodeResponse(t, resp, &filtered)
	if len(filtered.Data.Events) != 10 || !filtered.Data.HasMore {
		t.Fatalf("filtered page = %d events, has_more=%v", len(filtered.Data.Events), filtered.Data.HasMore)
	}
}
