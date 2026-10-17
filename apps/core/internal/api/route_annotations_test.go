package api

import (
	"orion/core/internal/config"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// undocumentedRoutes are registered routes that intentionally have no OpenAPI
// annotation (static assets, feeds, tokens in the path). Adding a route here
// needs a reason; everything else must carry a @Router annotation.
var undocumentedRoutes = map[string]string{
	"POST /v1/heartbeats/:p":            "heartbeat ping, token in path",
	"GET /":                             "SPA entry",
	"GET /feed.atom":                    "atom feed",
	"GET /status/:p/feed.atom":          "atom feed",
	"GET /swagger/*any":                 "swagger UI",
	"GET /alerts/email/confirm/:p":      "HTML opt-in page, signed token in path",
	"POST /alerts/email/confirm/:p":     "HTML opt-in action, signed token in path",
	"GET /alerts/email/unsubscribe/:p":  "HTML unsubscribe page, signed token in path",
	"POST /alerts/email/unsubscribe/:p": "unsubscribe action and RFC 8058 one-click, signed token in path",
	"GET /metrics":                      "Prometheus text exposition, not JSON",
}

var routerAnnotation = regexp.MustCompile(`@Router\s+(\S+)\s+\[(\w+)\]`)
var pathParam = regexp.MustCompile(`\{([^}]+)\}`)
var ginParam = regexp.MustCompile(`:[A-Za-z_]+`)

// normalizeRoute makes parameter names irrelevant: gin's :id and OpenAPI's {agent_id} both become :p.
func normalizeRoute(method, path string) string {
	return strings.ToUpper(method) + " " + ginParam.ReplaceAllString(pathParam.ReplaceAllString(path, ":p"), ":p")
}

func TestRegisteredRoutesMatchOpenAPIAnnotations(t *testing.T) {
	server := setupTestServerWithConfig(t, &config.Config{EnableSwagger: true})

	annotated := map[string]bool{}
	files, _ := filepath.Glob("*.go")
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range routerAnnotation.FindAllStringSubmatch(string(data), -1) {
			annotated[normalizeRoute(m[2], m[1])] = true
		}
	}

	registered := map[string]bool{}
	for _, route := range server.router.Routes() {
		registered[normalizeRoute(route.Method, route.Path)] = true
	}

	for route := range undocumentedRoutes {
		if !registered[route] {
			t.Errorf("undocumentedRoutes lists %q but no such route is registered; remove it", route)
		}
	}
	var missing, stale []string
	for route := range registered {
		if !annotated[route] {
			if _, ok := undocumentedRoutes[route]; !ok {
				missing = append(missing, route)
			}
		}
	}
	for route := range annotated {
		if !registered[route] {
			stale = append(stale, route)
		}
	}
	sort.Strings(missing)
	sort.Strings(stale)
	if len(missing) > 0 {
		t.Errorf("registered routes without a @Router annotation (annotate them or add to undocumentedRoutes with a reason):\n  %s", strings.Join(missing, "\n  "))
	}
	if len(stale) > 0 {
		t.Errorf("@Router annotations with no registered route:\n  %s", strings.Join(stale, "\n  "))
	}
}
