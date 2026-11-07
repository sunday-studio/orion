package service

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"orion/core/internal/db"
	"sort"
	"strings"
	"time"
)

// ServiceNameSource lists the OpenTelemetry service.name values recently seen in each app's logs
// (app id to names). *logstore.Store implements it.
type ServiceNameSource interface {
	RecentServiceNames(since time.Time, scanLimit int) (map[string][]string, error)
}

const (
	suggestionLogWindow    = 24 * time.Hour
	suggestionLogScanLimit = 5000
	suggestionMinPrefixLen = 4  // shorter keys only match exactly, so "api" never claims "api-gateway"
	suggestionMaxGroups    = 50 // a screen cannot use more
	suggestionMaxMembers   = 20
)

// matchKey is a name a group answers to, and where that name came from.
type matchKey struct {
	key     string
	against string
	label   string // the original text, for the reason
}

type suggestionGroup struct {
	out  ServiceSuggestion
	keys []matchKey
}

// suggestionCandidate is an unlinked thing and the forms of it that can match a key: a monitor
// has its name and, for Core monitors, its target host.
type suggestionCandidate struct {
	member ServiceSuggestionMember
	forms  []candidateForm
}

type candidateForm struct {
	matchedOn string
	display   string   // the original text, for the reason
	names     []string // normalised strings compared with a group's keys
}

// matchStrength is 2 for an exact match, 1 for a hyphen-delimited prefix, 0 for none.
func matchStrength(key, candidate string) int {
	switch {
	case key == "" || candidate == "":
		return 0
	case key == candidate:
		return 2
	case len(key) >= suggestionMinPrefixLen && strings.HasPrefix(candidate, key+"-"):
		return 1
	}
	return 0
}

// Suggest proposes links. With serviceID it proposes what could join that service; without, it
// proposes what could join each existing service and which unlinked apps could start a new one.
// It never changes anything.
func (c *ServiceCatalog) Suggest(serviceID string) ([]ServiceSuggestion, error) {
	var services []db.Service
	q := c.db.Order("name ASC")
	if serviceID != "" {
		q = q.Where("id = ?", serviceID)
	}
	if err := q.Find(&services).Error; err != nil {
		return nil, err
	}
	if serviceID != "" && len(services) == 0 {
		return nil, serviceNotFound("Service not found")
	}

	in, err := c.loadSuggestionInput()
	if err != nil {
		return nil, err
	}

	var groups []*suggestionGroup
	bySlug := map[string]bool{}
	for _, sv := range services {
		bySlug[sv.Slug] = true
		g := &suggestionGroup{out: ServiceSuggestion{ServiceID: sv.ID, Name: sv.Name, Slug: sv.Slug, Environment: sv.Environment}}
		g.keys = append(g.keys, matchKey{ServiceSlug(sv.Name), "service_name", sv.Name}, matchKey{sv.Slug, "service_slug", sv.Slug})
		for _, a := range in.linkedApps[sv.ID] {
			g.keys = append(g.keys, in.appKeys(a)...)
		}
		groups = append(groups, g)
	}

	taken := map[string]bool{} // candidates already given to a group
	// Pass 1: unlinked apps that answer to an existing service.
	for _, app := range in.apps {
		cand := in.appCandidate(app)
		if g, key, form, _ := bestGroup(groups, cand, true); g != nil {
			addMember(g, cand, form, key, 2)
			taken[app.ID] = true
		}
	}
	// Pass 2: every other unlinked app can start a new service.
	if serviceID == "" {
		for _, app := range in.apps {
			if taken[app.ID] || bySlug[app.Slug] {
				continue
			}
			g := &suggestionGroup{out: ServiceSuggestion{Name: app.Name, Slug: app.Slug, Environment: app.Environment}, keys: in.appKeys(app)}
			g.out.Members = append(g.out.Members, ServiceSuggestionMember{Kind: ServiceLinkApp, ID: app.ID, Name: app.Name,
				MatchedOn: "app_slug", Against: "app_slug", Confidence: "high",
				Reason: fmt.Sprintf("App %q sends the logs; its slug %q names the new service", app.Name, app.Slug)})
			groups = append(groups, g)
		}
	}
	// Pass 3: monitors and resources go to the group they match best.
	for _, cand := range append(in.monitorCandidates(), in.resourceCandidates()...) {
		if g, key, form, strength := bestGroup(groups, cand, false); g != nil {
			addMember(g, cand, form, key, strength)
		}
	}

	out := make([]ServiceSuggestion, 0, len(groups))
	for _, g := range groups {
		minMembers := 1
		if g.out.ServiceID == "" {
			minMembers = 2 // the anchor app alone is not a suggestion
		}
		if len(g.out.Members) < minMembers {
			continue
		}
		if len(g.out.Members) > suggestionMaxMembers {
			g.out.Members = g.out.Members[:suggestionMaxMembers]
		}
		out = append(out, g.out)
		if len(out) >= suggestionMaxGroups {
			break
		}
	}
	return out, nil
}

// bestGroup finds the group, key and candidate form with the strongest match (a longer key breaks
// ties, then the earlier group so existing services win, then the earlier form so a name beats a
// host). exactOnly is used for apps, which never match by prefix.
func bestGroup(groups []*suggestionGroup, cand suggestionCandidate, exactOnly bool) (*suggestionGroup, matchKey, candidateForm, int) {
	var best *suggestionGroup
	var bestKey matchKey
	var bestForm candidateForm
	bestStrength := 0
	for _, g := range groups {
		for _, k := range g.keys {
			for _, f := range cand.forms {
				for _, name := range f.names {
					s := matchStrength(k.key, name)
					if s == 0 || (exactOnly && s < 2) {
						continue
					}
					if s > bestStrength || (s == bestStrength && len(k.key) > len(bestKey.key)) {
						best, bestKey, bestForm, bestStrength = g, k, f, s
					}
				}
			}
		}
	}
	return best, bestKey, bestForm, bestStrength
}

func addMember(g *suggestionGroup, cand suggestionCandidate, form candidateForm, key matchKey, strength int) {
	m := cand.member
	m.MatchedOn, m.Against = form.matchedOn, key.against
	how, confidence := "starts with", "medium"
	if strength >= 2 {
		how, confidence = "matches", "high"
	}
	m.Confidence = confidence
	m.Reason = fmt.Sprintf("%s %q %s %s %q", describeForm(m.Kind, form.matchedOn), form.display, how, againstText(key.against), key.label)
	g.out.Members = append(g.out.Members, m)
	sort.SliceStable(g.out.Members, func(i, j int) bool { return memberOrder(g.out.Members[i]) < memberOrder(g.out.Members[j]) })
}

func memberOrder(m ServiceSuggestionMember) string {
	rank := map[string]string{ServiceLinkApp: "0", ServiceLinkMonitor: "1", ServiceLinkResource: "2"}[m.Kind]
	return rank + strings.ToLower(m.Name)
}

func againstText(against string) string {
	switch against {
	case "otel_service_name":
		return "the OpenTelemetry service.name seen in the app's logs"
	case "app_slug":
		return "the app slug"
	case "service_name":
		return "the service name"
	}
	return "the service slug"
}

func describeForm(kind, matchedOn string) string {
	switch matchedOn {
	case "monitor_host":
		return "Monitor target host"
	case "monitor_name":
		return "Monitor name"
	case "app_service_name":
		return "A service.name in this app's logs"
	case "app_slug":
		return "App slug"
	}
	return strings.ToUpper(kind[:1]) + kind[1:] + " name"
}

// suggestionInput is the unlinked things and the keys of linked apps, loaded once per request.
type suggestionInput struct {
	apps       []db.App
	linkedApps map[string][]db.App
	monitors   []db.Monitor
	hosts      map[string]string // monitor id to target host
	resources  []db.NodeAppResource
	otel       map[string][]string // app id to service.name values
}

func (c *ServiceCatalog) loadSuggestionInput() (*suggestionInput, error) {
	in := &suggestionInput{linkedApps: map[string][]db.App{}, hosts: map[string]string{}}
	var all []db.App
	if err := c.db.Select("id", "name", "slug", "environment", "service_id").Find(&all).Error; err != nil {
		return nil, err
	}
	for _, a := range all {
		if a.ServiceID != nil {
			in.linkedApps[*a.ServiceID] = append(in.linkedApps[*a.ServiceID], a)
		} else {
			in.apps = append(in.apps, a)
		}
	}
	if err := c.db.Select("id", "name", "type").Where("service_id IS NULL AND system = ? AND lifecycle <> ?", false, monitorLifecycleDeleted).
		Order("name ASC").Find(&in.monitors).Error; err != nil {
		return nil, err
	}
	var configs []db.CoreMonitorConfig
	if err := c.db.Select("monitor_id", "config_json").Find(&configs).Error; err != nil {
		return nil, err
	}
	for _, cfg := range configs {
		if host := monitorTargetHost(cfg.ConfigJSON); host != "" {
			in.hosts[cfg.MonitorID] = host
		}
	}
	if err := c.db.Select("node_id", "key", "kind", "name").Where("service_id IS NULL AND kind IN ?", []string{"process", "unit", "container"}).
		Order("name ASC").Find(&in.resources).Error; err != nil {
		return nil, err
	}
	if c.names != nil {
		// Best effort: a failing log store only costs the service.name rule.
		if names, err := c.names.RecentServiceNames(time.Now().UTC().Add(-suggestionLogWindow), suggestionLogScanLimit); err == nil {
			in.otel = names
		}
	}
	return in, nil
}

// monitorTargetHost reads the host a Core monitor checks from its config ("url" or "host").
func monitorTargetHost(configJSON string) string {
	var cfg struct {
		URL  string `json:"url"`
		Host string `json:"host"`
	}
	if json.Unmarshal([]byte(configJSON), &cfg) != nil {
		return ""
	}
	host := strings.TrimSpace(cfg.Host)
	if cfg.URL != "" {
		if u, err := url.Parse(strings.TrimSpace(cfg.URL)); err == nil {
			host = u.Hostname()
		}
	}
	host = strings.ToLower(strings.Trim(host, "[]."))
	if host == "" || host == "localhost" || net.ParseIP(host) != nil {
		return ""
	}
	return host
}

func (in *suggestionInput) appKeys(a db.App) []matchKey {
	keys := []matchKey{{ServiceSlug(a.Slug), "app_slug", a.Slug}}
	for _, name := range in.otel[a.ID] {
		if k := ServiceSlug(name); k != "" {
			keys = append(keys, matchKey{k, "otel_service_name", name})
		}
	}
	return keys
}

func (in *suggestionInput) appCandidate(a db.App) suggestionCandidate {
	cand := suggestionCandidate{member: ServiceSuggestionMember{Kind: ServiceLinkApp, ID: a.ID, Name: a.Name},
		forms: []candidateForm{{matchedOn: "app_slug", display: a.Slug, names: []string{ServiceSlug(a.Slug)}}}}
	for _, name := range in.otel[a.ID] {
		if k := ServiceSlug(name); k != "" {
			cand.forms = append(cand.forms, candidateForm{matchedOn: "app_service_name", display: name, names: []string{k}})
		}
	}
	return cand
}

func (in *suggestionInput) monitorCandidates() []suggestionCandidate {
	out := make([]suggestionCandidate, 0, len(in.monitors))
	for _, m := range in.monitors {
		cand := suggestionCandidate{member: ServiceSuggestionMember{Kind: ServiceLinkMonitor, ID: m.ID, Name: m.Name},
			forms: []candidateForm{{matchedOn: "monitor_name", display: m.Name, names: []string{ServiceSlug(m.Name)}}}}
		if host := in.hosts[m.ID]; host != "" {
			label, _, _ := strings.Cut(host, ".")
			cand.forms = append(cand.forms, candidateForm{matchedOn: "monitor_host", display: host, names: []string{ServiceSlug(label)}})
		}
		out = append(out, cand)
	}
	return out
}

func (in *suggestionInput) resourceCandidates() []suggestionCandidate {
	out := make([]suggestionCandidate, 0, len(in.resources))
	for _, r := range in.resources {
		name := strings.TrimSuffix(strings.ToLower(r.Name), ".service")
		out = append(out, suggestionCandidate{member: ServiceSuggestionMember{Kind: ServiceLinkResource, NodeID: r.NodeID, Key: r.Key, Name: r.Name},
			forms: []candidateForm{{matchedOn: "resource_name", display: r.Name, names: []string{ServiceSlug(name)}}}})
	}
	return out
}
