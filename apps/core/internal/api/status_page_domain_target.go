package api

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"strings"
	"time"
)

const (
	domainRecordCNAME = "CNAME"
	domainRecordA     = "A"
	domainRecordAAAA  = "AAAA"
)

// domainTarget describes where a custom domain must point. A hostname needs a CNAME record; an IP address
// cannot be a CNAME value, so it needs an A (IPv4) or AAAA (IPv6) record instead.
type domainTarget struct {
	value      string
	recordType string
	ip         netip.Addr
}

func parseDomainTarget(target string) domainTarget {
	target = strings.ToLower(strings.TrimSpace(target))
	if ip, err := netip.ParseAddr(target); err == nil {
		ip = ip.Unmap()
		if ip.Is4() {
			return domainTarget{value: ip.String(), recordType: domainRecordA, ip: ip}
		}
		return domainTarget{value: ip.String(), recordType: domainRecordAAAA, ip: ip}
	}
	return domainTarget{value: target, recordType: domainRecordCNAME}
}

// warning explains why a target that only works inside one network will not serve the public internet.
// It does not block anything: an internal status page on a tailnet is a legitimate setup.
func (t domainTarget) warning() string {
	switch {
	case t.value == "localhost" || strings.HasSuffix(t.value, ".localhost"):
		return "This address is this machine only, so the custom domain will only work on this machine."
	case !t.ip.IsValid():
		return ""
	case t.ip.IsLoopback() || t.ip.IsPrivate() || t.ip.IsLinkLocalUnicast() || t.ip.IsUnspecified() || cgnat.Contains(t.ip):
		return "This address is not publicly reachable (private, loopback or tailnet range), so the custom domain will only work inside that network. Set ORION_PUBLIC_URL or ORION_STATUS_PAGES_CNAME_TARGET to a public hostname or address to serve it on the internet."
	}
	return ""
}

var cgnat = netip.MustParsePrefix("100.64.0.0/10")

// checkAddressRecord verifies a domain whose target is an IP: the domain must resolve to that address.
func (s *Server) checkAddressRecord(ctx context.Context, domain string, target domainTarget) (string, string) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	hosts, err := s.lookupHost(ctx, domain)
	var dnsErr *net.DNSError
	switch {
	case errors.As(err, &dnsErr) && dnsErr.IsNotFound:
		return domainStatusPending, "Record not found yet"
	case err != nil:
		return domainStatusPending, "DNS lookup failed, try again in a minute"
	}
	found := false
	for _, host := range hosts {
		ip, parseErr := netip.ParseAddr(strings.TrimSpace(host))
		if parseErr != nil {
			continue
		}
		found = true
		if ip.Unmap() == target.ip {
			return domainStatusVerified, ""
		}
	}
	if !found {
		return domainStatusPending, "Record not found yet"
	}
	return domainStatusFailed, "Record points somewhere else"
}
