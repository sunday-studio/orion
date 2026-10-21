package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"orion/core/internal/config"
	"orion/core/internal/utils"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func rawRequest(server *Server, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)
	return rec
}

// S5: oversized bodies get 413 on report and log endpoints.
func TestOversizedBodyReturns413(t *testing.T) {
	server := setupTestServer(t)
	node := registerTestNode(t, server)
	big := bytes.Repeat([]byte("a"), 6<<20) // 6 MiB > both caps

	for _, path := range []string{"/v1/nodes/" + node.Data.NodeID + "/report", "/v1/nodes/" + node.Data.NodeID + "/logs/batch", "/v1/register"} {
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(big))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+node.Data.Token)
		if rec := rawRequest(server, req); rec.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("%s status = %d, want 413", path, rec.Code)
		}
	}
}

func TestLogBatchAllowsLargerBodyThanDefault(t *testing.T) {
	if maxBodyBytesFor("/v1/nodes/:node_id/logs/batch", "application/json") != serviceLogMaxBodyBytes || maxBodyBytesFor("/v1/register", "") != defaultMaxBodyBytes || maxBodyBytesFor("/v1/ingest/logs", "application/x-ndjson") != serviceLogMaxBodyBytes || maxBodyBytesFor("/v1/ingest/logs", "application/json") != defaultMaxBodyBytes {
		t.Fatal("log batches must get the 5 MiB cap and everything else 1 MiB")
	}
}

func TestUndeclaredLengthBodyIsCapped(t *testing.T) {
	server := setupTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/register", &readerWithoutLength{r: bytes.NewReader(bytes.Repeat([]byte("{"), 2<<20))})
	req.ContentLength = -1
	req.Header.Set("Content-Type", "application/json")
	if rec := rawRequest(server, req); rec.Code == http.StatusOK || rec.Code == http.StatusCreated {
		t.Fatalf("2 MiB chunked body accepted with status %d", rec.Code)
	}
}

type readerWithoutLength struct{ r *bytes.Reader }

func (r *readerWithoutLength) Read(p []byte) (int, error) { return r.r.Read(p) }

// S6: rotating X-Forwarded-For does not evade the login limiter by default.
func TestLoginLimiterIgnoresSpoofedForwardedFor(t *testing.T) {
	server := setupTestServerWithConfig(t, &config.Config{
		AdminUsername: "admin", AdminPassword: "right-password", JWTSecret: "secret-secret-secret-secret-1234", FrontendAuthOn: true,
		LoginRateLimitAttempts: 3, LoginRateLimitWindowSecs: 60,
	})
	var last int
	for i := 0; i < 6; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", strings.NewReader(`{"username":"admin","password":"wrong"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Forwarded-For", "198.51.100."+string(rune('1'+i)))
		req.RemoteAddr = "203.0.113.9:5555"
		last = rawRequest(server, req).Code
	}
	if last != http.StatusTooManyRequests {
		t.Fatalf("6th attempt with rotating X-Forwarded-For status = %d, want 429", last)
	}
}

func TestLoginLimiterHonoursForwardedForFromTrustedProxy(t *testing.T) {
	server := setupTestServerWithConfig(t, &config.Config{
		AdminUsername: "admin", AdminPassword: "right-password", JWTSecret: "secret-secret-secret-secret-1234", FrontendAuthOn: true,
		LoginRateLimitAttempts: 2, LoginRateLimitWindowSecs: 60, TrustedProxies: []string{"10.0.0.1"},
	})
	try := func(client string) int {
		req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", strings.NewReader(`{"username":"admin","password":"wrong"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Forwarded-For", client)
		req.RemoteAddr = "10.0.0.1:1234"
		return rawRequest(server, req).Code
	}
	try("198.51.100.1")
	try("198.51.100.1")
	if try("198.51.100.1") != http.StatusTooManyRequests {
		t.Fatal("same real client behind a trusted proxy should be limited")
	}
	if try("198.51.100.2") == http.StatusTooManyRequests {
		t.Fatal("a different real client behind the trusted proxy must not share the budget")
	}
}

func TestSuccessfulLoginResetsLimiter(t *testing.T) {
	server := setupTestServerWithConfig(t, &config.Config{
		AdminUsername: "admin", AdminPassword: "right-password", JWTSecret: "secret-secret-secret-secret-1234", FrontendAuthOn: true,
		LoginRateLimitAttempts: 3, LoginRateLimitWindowSecs: 60,
	})
	post := func(pass string) int {
		req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", strings.NewReader(`{"username":"admin","password":"`+pass+`"}`))
		req.Header.Set("Content-Type", "application/json")
		return rawRequest(server, req).Code
	}
	post("wrong")
	post("wrong")
	if post("right-password") != http.StatusOK {
		t.Fatal("valid login within budget should succeed")
	}
	post("wrong")
	post("wrong")
	if post("right-password") != http.StatusOK {
		t.Fatal("budget should have reset after the successful login")
	}
}

func TestSwaggerIsOffByDefaultAndSecurityHeadersPresent(t *testing.T) {
	server := setupTestServer(t)
	if rec := rawRequest(server, httptest.NewRequest(http.MethodGet, "/swagger/index.html", nil)); rec.Code != http.StatusNotFound {
		t.Fatalf("swagger status = %d, want 404 when ORION_ENABLE_SWAGGER is unset", rec.Code)
	}
	rec := rawRequest(server, httptest.NewRequest(http.MethodGet, "/health", nil))
	for header, want := range map[string]string{"X-Content-Type-Options": "nosniff", "X-Frame-Options": "SAMEORIGIN", "Referrer-Policy": "strict-origin-when-cross-origin"} {
		if got := rec.Header().Get(header); got != want {
			t.Fatalf("%s = %q, want %q", header, got, want)
		}
	}
	if rec.Header().Get("Content-Security-Policy") == "" {
		t.Fatal("CSP missing on non-API response")
	}
	api := rawRequest(server, httptest.NewRequest(http.MethodGet, "/v1/nodes", nil))
	if api.Header().Get("Content-Security-Policy") != "" {
		t.Fatal("CSP should not be set on the JSON API")
	}
}

func TestRequestIDFromRejectsUnsafeValues(t *testing.T) {
	if got := requestIDFrom("trace-123_ok.4"); got != "trace-123_ok.4" {
		t.Fatalf("safe id was replaced: %q", got)
	}
	for _, bad := range []string{"line1\nline2", strings.Repeat("a", 65), "has space", "<script>"} {
		if got := requestIDFrom(bad); got == bad {
			t.Fatalf("unsafe id %q accepted", bad)
		}
	}
}

// S7: access logs record the route template, never path tokens; 5xx bodies hide causes.
func TestAccessLogOmitsPathTokensAndErrorBodiesHideCauses(t *testing.T) {
	var out bytes.Buffer
	logger := utils.NewLoggerWithWriter(&out)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(AccessLogMiddleware(logger))
	router.POST("/v1/heartbeats/:token", func(c *gin.Context) {
		utils.InternalError(c, "Failed", http.ErrAbortHandler)
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/heartbeats/SECRET-HEARTBEAT-TOKEN", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if strings.Contains(out.String(), "SECRET-HEARTBEAT-TOKEN") {
		t.Fatalf("access log leaked the heartbeat token: %s", out.String())
	}
	if !strings.Contains(out.String(), "/v1/heartbeats/:token") {
		t.Fatalf("access log missing route template: %s", out.String())
	}
	if strings.Contains(rec.Body.String(), http.ErrAbortHandler.Error()) {
		t.Fatalf("5xx body exposed the internal cause: %s", rec.Body.String())
	}
}
