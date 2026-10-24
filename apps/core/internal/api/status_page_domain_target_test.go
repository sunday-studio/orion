package api

import (
	"context"
	"net"
	"net/http"
	"orion/core/internal/db"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestParseDomainTargetPicksTheRecordType(t *testing.T) {
	for target, want := range map[string]string{"pages.orion.example.com": "CNAME", "203.0.113.7": "A", "2001:db8::7": "AAAA", "::ffff:203.0.113.7": "A", " Pages.Example.COM ": "CNAME"} {
		if got := parseDomainTarget(target).recordType; got != want {
			t.Errorf("%q: type = %s, want %s", target, got, want)
		}
	}
}

func TestDomainTargetWarningOnlyForNonPublicAddresses(t *testing.T) {
	for target, warn := range map[string]bool{"100.100.108.49": true, "10.1.2.3": true, "172.20.0.5": true, "192.168.1.9": true, "127.0.0.1": true, "fd00::1": true, "localhost": true,
		"203.0.113.7": false, "8.8.8.8": false, "2001:4860:4860::8888": false, "status.example.com": false} {
		if got := parseDomainTarget(target).warning() != ""; got != warn {
			t.Errorf("%q: warning = %v, want %v", target, got, warn)
		}
	}
}

func TestStatusPageDomainWithAnAddressTargetUsesAAndAAAARecordsAndVerifiesByAddress(t *testing.T) {
	cases := []struct {
		name, target string
		hosts        []string
		lookupErr    error
		wantType     string
		wantStatus   string
		wantText     string
	}{
		{"IPv4 match", "203.0.113.7", []string{"203.0.113.7"}, nil, "A", "verified", ""},
		{"IPv6 match", "2001:db8::7", []string{"2001:db8:0:0:0:0:0:7"}, nil, "AAAA", "verified", ""},
		{"one of several addresses matches", "203.0.113.7", []string{"198.51.100.1", "203.0.113.7"}, nil, "A", "verified", ""},
		{"no record", "203.0.113.7", nil, &net.DNSError{Err: "no such host", IsNotFound: true}, "A", "pending", "Record not found yet"},
		{"empty answer", "203.0.113.7", []string{}, nil, "A", "pending", "Record not found yet"},
		{"wrong address", "203.0.113.7", []string{"198.51.100.1"}, nil, "A", "failed", "Record points somewhere else"},
		{"resolver failure", "203.0.113.7", nil, &net.DNSError{Err: "timeout", IsTimeout: true}, "A", "pending", "DNS lookup failed, try again in a minute"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := setupTestServer(t)
			server.cfg.StatusPagesCNAMETarget = tc.target
			page, _, _ := createPublishedStatusPageForSubscriberTest(t, server, "addr-status")
			server.lookupHost = func(ctx context.Context, host string) ([]string, error) { return tc.hosts, tc.lookupErr }
			server.lookupCNAME = func(ctx context.Context, host string) (string, error) {
				t.Fatalf("an address target must not use a CNAME lookup")
				return "", nil
			}
			if r := performJSONRequest(t, server, http.MethodPut, "/v1/status-pages/"+page.ID, gin.H{"custom_domain": "status.acme.test"}, ""); r.Code != http.StatusOK {
				t.Fatalf("set domain = %d %s", r.Code, r.Body.String())
			}
			var env struct {
				Data StatusPageDomainResponse `json:"data"`
			}
			server.db.Model(&db.StatusPage{}).Where("id = ?", page.ID).Update("domain_checked_at", time.Now().Add(-time.Minute))
			decodeResponse(t, performJSONRequest(t, server, http.MethodPost, "/v1/status-pages/"+page.ID+"/domain/verify", nil, ""), &env)
			d := env.Data
			if len(d.Records) != 1 || d.Records[0].Type != tc.wantType || d.Records[0].Host != "status.acme.test" {
				t.Fatalf("records = %+v, want one %s record", d.Records, tc.wantType)
			}
			if d.Status != tc.wantStatus || d.StatusText != firstNonEmpty(tc.wantText, "Verified") {
				t.Fatalf("status = %q (%q), want %q (%q)", d.Status, d.StatusText, tc.wantStatus, tc.wantText)
			}
		})
	}
}

func TestStatusPageDomainResponseWarnsAboutAPrivateTargetButStillShowsTheRecord(t *testing.T) {
	server := setupTestServer(t)
	server.cfg.StatusPagesCNAMETarget = ""
	server.cfg.SetPublicURLOverride("http://100.100.108.49:8999")
	page, _, _ := createPublishedStatusPageForSubscriberTest(t, server, "tailnet-status")
	server.db.Model(&db.StatusPage{}).Where("id = ?", page.ID).Update("custom_domain", "status.acme.test")
	var env struct {
		Data StatusPageDomainResponse `json:"data"`
	}
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/status-pages/"+page.ID+"/domain", nil, ""), &env)
	d := env.Data
	if len(d.Records) != 1 || d.Records[0].Type != "A" || d.Records[0].Value != "100.100.108.49" {
		t.Fatalf("records = %+v, want an A record for the IP, never a CNAME", d.Records)
	}
	if d.TargetWarning == "" {
		t.Fatalf("a tailnet address needs a target_warning")
	}
	server.cfg.SetPublicURLOverride("https://orion.example.com")
	env.Data = StatusPageDomainResponse{}
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/status-pages/"+page.ID+"/domain", nil, ""), &env)
	if env.Data.Records[0].Type != "CNAME" || env.Data.TargetWarning != "" {
		t.Fatalf("a public hostname gives a CNAME and no warning: %+v", env.Data)
	}
}
