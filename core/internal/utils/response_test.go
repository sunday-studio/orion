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
		data       interface{}
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
