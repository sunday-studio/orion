package utils

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSuccessResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		statusCode int
		message    string
		data       any
	}{
		{name: "ok with data", statusCode: 200, message: "OK", data: map[string]string{"id": "x"}},
		{name: "created with nil data", statusCode: 201, message: "Created", data: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			SuccessResponse(c, tt.statusCode, tt.message, tt.data)
			if w.Code != tt.statusCode {
				t.Errorf("status = %d, want %d", w.Code, tt.statusCode)
			}
			var got APIResponse
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Fatalf("json.Unmarshal: %v", err)
			}
			if !got.Success || got.Message != tt.message {
				t.Errorf("body = %+v", got)
			}
		})
	}
}

func TestErrorResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	ErrorResponse(c, 400, "Bad request", nil)

	if w.Code != 400 {
		t.Errorf("status = %d, want 400", w.Code)
	}
	var got APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got.Success || got.Message != "Bad request" {
		t.Errorf("body = %+v", got)
	}
}

func TestNewPaginationMeta(t *testing.T) {
	meta := NewPaginationMeta(50, 10, 0, 10)
	if meta.RangeStart != 1 || meta.RangeEnd != 10 {
		t.Fatalf("range = %d-%d, want 1-10", meta.RangeStart, meta.RangeEnd)
	}
	if meta.CurrentPage != 1 || meta.TotalPages != 5 {
		t.Fatalf("pages = %d/%d, want 1/5", meta.CurrentPage, meta.TotalPages)
	}
	if !meta.HasNext || meta.HasPrevious {
		t.Fatalf("next/previous = %t/%t, want true/false", meta.HasNext, meta.HasPrevious)
	}
	if meta.NextOffset == nil || *meta.NextOffset != 10 {
		t.Fatalf("next offset = %v, want 10", meta.NextOffset)
	}

	meta = NewPaginationMeta(50, 10, 40, 10)
	if meta.RangeStart != 41 || meta.RangeEnd != 50 {
		t.Fatalf("last range = %d-%d, want 41-50", meta.RangeStart, meta.RangeEnd)
	}
	if meta.HasNext || !meta.HasPrevious {
		t.Fatalf("last next/previous = %t/%t, want false/true", meta.HasNext, meta.HasPrevious)
	}
	if meta.PreviousOffset == nil || *meta.PreviousOffset != 30 {
		t.Fatalf("previous offset = %v, want 30", meta.PreviousOffset)
	}

	meta = NewPaginationMeta(0, 10, 0, 0)
	if meta.RangeStart != 0 || meta.RangeEnd != 0 || meta.TotalPages != 0 {
		t.Fatalf("empty meta = %+v, want empty range and zero total pages", meta)
	}
}
