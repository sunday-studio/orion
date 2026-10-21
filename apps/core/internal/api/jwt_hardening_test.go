package api

import (
	"net/http"
	"net/http/httptest"
	"orion/core/internal/config"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestFrontendAuthRejectsWeakTokens(t *testing.T) {
	const secret = "secret-secret-secret-secret-1234"
	server := setupTestServerWithConfig(t, &config.Config{AdminUsername: "admin", AdminPassword: "pw", JWTSecret: secret, FrontendAuthOn: true})
	get := func(token string) int {
		req := httptest.NewRequest(http.MethodGet, "/v1/nodes", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		return rawRequest(server, req).Code
	}
	sign := func(method jwt.SigningMethod, claims jwt.MapClaims) string {
		s, err := jwt.NewWithClaims(method, claims).SignedString([]byte(secret))
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	valid := jwt.MapClaims{"sub": "admin", "exp": time.Now().Add(time.Hour).Unix()}
	if got := get(sign(jwt.SigningMethodHS256, valid)); got != http.StatusOK {
		t.Fatalf("valid token status = %d, want 200", got)
	}
	if got := get(sign(jwt.SigningMethodHS256, jwt.MapClaims{"sub": "admin"})); got != http.StatusUnauthorized {
		t.Fatalf("token without exp status = %d, want 401", got)
	}
	if got := get(sign(jwt.SigningMethodHS512, valid)); got != http.StatusUnauthorized {
		t.Fatalf("HS512 token status = %d, want 401 (only HS256 accepted)", got)
	}
	none, _ := jwt.NewWithClaims(jwt.SigningMethodNone, valid).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if got := get(none); got != http.StatusUnauthorized {
		t.Fatalf("alg=none token status = %d, want 401", got)
	}
	expired := jwt.MapClaims{"sub": "admin", "exp": time.Now().Add(-time.Minute).Unix()}
	if got := get(sign(jwt.SigningMethodHS256, expired)); got != http.StatusUnauthorized {
		t.Fatalf("expired token status = %d, want 401", got)
	}
}
