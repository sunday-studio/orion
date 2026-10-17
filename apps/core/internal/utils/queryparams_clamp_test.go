package utils

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func ginContext(query string) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/?"+query, nil)
	return c
}

// S8: hostile limit and offset are clamped centrally.
func TestQueryIntClampsLimitAndOffset(t *testing.T) {
	for query, want := range map[string]struct{ limit, offset int }{
		"limit=1000000000&offset=1000000000": {MaxQueryLimit, MaxQueryOffset},
		"limit=50&offset=10":                 {50, 10},
		"limit=-5&offset=abc":                {25, 0},
	} {
		c := ginContext(query)
		if got := QueryInt(c, "limit", 25); got != want.limit {
			t.Fatalf("%s: limit = %d, want %d", query, got, want.limit)
		}
		if got := QueryInt(c, "offset", 0); got != want.offset {
			t.Fatalf("%s: offset = %d, want %d", query, got, want.offset)
		}
	}
	if got := QueryInt(ginContext("days=100000"), "days", 7); got != 100000 {
		t.Fatalf("unrelated keys must not be clamped, got %d", got)
	}
}
