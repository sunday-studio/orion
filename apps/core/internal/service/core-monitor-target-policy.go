package service

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"orion/core/internal/config"
	"strings"
)

type CoreMonitorTargetPolicy struct {
	AllowPrivateTargets bool
}

func NewCoreMonitorTargetPolicy(cfg *config.Config) CoreMonitorTargetPolicy {
	policy := CoreMonitorTargetPolicy{}
	if cfg != nil {
		policy.AllowPrivateTargets = cfg.CoreMonitorAllowPrivateTargets
	}
	return policy
}

func (p CoreMonitorTargetPolicy) ValidateURL(rawURL string, field string) error {
	parsedURL, err := parseCoreMonitorURL(rawURL, field)
	if err != nil {
		return err
	}
	if parsedURL.User != nil {
		return fmt.Errorf("%w: %s must not include username or password", ErrCoreManagedMonitorValidation, field)
	}
	return p.ValidateHost(parsedURL.Hostname(), field+" host")
}

func (p CoreMonitorTargetPolicy) ValidateHost(host string, field string) error {
	host = normalizeCoreMonitorPolicyHost(host)
	if host == "" {
		return fmt.Errorf("%w: %s is required", ErrCoreManagedMonitorValidation, field)
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		if p.AllowPrivateTargets {
			return nil
		}
		return fmt.Errorf("%w: %s targets localhost, which is blocked for Core monitors", ErrCoreManagedMonitorValidation, field)
	}
	if addr, ok := parseCoreMonitorPolicyAddr(host); ok {
		if reason := p.blockedAddrReason(addr); reason != "" {
			return fmt.Errorf("%w: %s target %s is blocked for Core monitors", ErrCoreManagedMonitorValidation, field, reason)
		}
	}
	return nil
}

func (p CoreMonitorTargetPolicy) CheckRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return fmt.Errorf("stopped after 10 redirects")
	}
	if req == nil || req.URL == nil {
		return fmt.Errorf("redirect target is missing")
	}
	if err := p.ValidateURL(req.URL.String(), "redirect url"); err != nil {
		return err
	}
	return nil
}

func (p CoreMonitorTargetPolicy) HTTPClient(base *http.Client) *http.Client {
	if base == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.DialContext = p.DialContext
		return &http.Client{Transport: transport, CheckRedirect: p.CheckRedirect}
	}
	client := *base
	if client.CheckRedirect == nil {
		client.CheckRedirect = p.CheckRedirect
	}
	if client.Transport == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.DialContext = p.DialContext
		client.Transport = transport
	} else if transport, ok := client.Transport.(*http.Transport); ok {
		policyTransport := transport.Clone()
		policyTransport.DialContext = p.DialContext
		client.Transport = policyTransport
	}
	return &client
}

func (p CoreMonitorTargetPolicy) DialContext(ctx context.Context, network string, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	host = normalizeCoreMonitorPolicyHost(host)
	if err := p.ValidateHost(host, "target host"); err != nil {
		return nil, err
	}

	var addrs []netip.Addr
	if addr, ok := parseCoreMonitorPolicyAddr(host); ok {
		addrs = []netip.Addr{addr}
	} else {
		resolved, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
		if err != nil {
			return nil, err
		}
		addrs = resolved
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("target host %s did not resolve", host)
	}

	var blockedReasons []string
	var firstErr error
	dialer := &net.Dialer{}
	for _, addr := range addrs {
		if reason := p.blockedAddrReason(addr); reason != "" {
			blockedReasons = append(blockedReasons, reason)
			continue
		}
		conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(addr.String(), port))
		if err == nil {
			return conn, nil
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	if len(blockedReasons) > 0 {
		return nil, fmt.Errorf("target host %s resolved to blocked Core monitor address: %s", host, strings.Join(blockedReasons, ", "))
	}
	if firstErr != nil {
		return nil, firstErr
	}
	return nil, fmt.Errorf("target host %s did not have an allowed address", host)
}

func (p CoreMonitorTargetPolicy) blockedAddrReason(addr netip.Addr) string {
	if !addr.IsValid() {
		return "invalid address"
	}
	if addr.IsUnspecified() {
		return "unspecified address"
	}
	if addr.IsLoopback() && !p.AllowPrivateTargets {
		return "loopback address"
	}
	if addr.IsLinkLocalUnicast() {
		return "link-local address"
	}
	if addr.IsLinkLocalMulticast() || addr.IsMulticast() {
		return "multicast address"
	}
	if isCoreMonitorMetadataAddress(addr) {
		return "metadata address"
	}
	if addr.IsPrivate() && !p.AllowPrivateTargets {
		return "private network address"
	}
	return ""
}

func parseCoreMonitorURL(rawURL string, field string) (*url.URL, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, fmt.Errorf("%w: %s is required", ErrCoreManagedMonitorValidation, field)
	}
	parsedURL, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return nil, fmt.Errorf("%w: %s is invalid: %v", ErrCoreManagedMonitorValidation, field, err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, fmt.Errorf("%w: %s scheme must be http or https", ErrCoreManagedMonitorValidation, field)
	}
	if parsedURL.Host == "" {
		return nil, fmt.Errorf("%w: %s host is required", ErrCoreManagedMonitorValidation, field)
	}
	return parsedURL, nil
}

func normalizeCoreMonitorPolicyHost(host string) string {
	host = strings.TrimSpace(host)
	host = strings.TrimPrefix(strings.TrimSuffix(host, "."), "[")
	host = strings.TrimSuffix(host, "]")
	if percent := strings.LastIndex(host, "%"); percent >= 0 {
		host = host[:percent]
	}
	return strings.ToLower(host)
}

func parseCoreMonitorPolicyAddr(host string) (netip.Addr, bool) {
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}, false
	}
	if addr.Is4In6() {
		addr = addr.Unmap()
	}
	return addr, true
}

func isCoreMonitorMetadataAddress(addr netip.Addr) bool {
	if addr.Is4In6() {
		addr = addr.Unmap()
	}
	return addr == netip.MustParseAddr("169.254.169.254") ||
		addr == netip.MustParseAddr("100.100.100.200") ||
		addr == netip.MustParseAddr("fd00:ec2::254")
}

func SanitizeCoreMonitorURL(rawURL string) string {
	parsedURL, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return strings.TrimSpace(rawURL)
	}
	parsedURL.User = nil
	parsedURL.RawQuery = ""
	parsedURL.Fragment = ""
	return parsedURL.String()
}

func CoreMonitorURLHost(rawURL string) string {
	parsedURL, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	return normalizeCoreMonitorPolicyHost(parsedURL.Hostname())
}
