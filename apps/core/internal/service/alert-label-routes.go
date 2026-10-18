package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"regexp"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
)

var (
	ErrLabelRouteInvalid  = errors.New("invalid label route")
	ErrLabelRouteNotFound = errors.New("label route not found")
	labelKeyPattern       = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_.-]{0,39}$`)
	labelValuePattern     = regexp.MustCompile(`^[A-Za-z0-9_.:/@+ -]{0,80}$`)
)

const maxLabelRoutes = 100

// LabelMatcher is one condition; Op is "=" or "!=".
type LabelMatcher struct {
	Key   string `json:"key"`
	Op    string `json:"op"`
	Value string `json:"value"`
}

// String reads "env=prod" or "env!=dev".
func (m LabelMatcher) String() string { return m.Key + m.Op + m.Value }

// DescribeMatchers reads "env=prod AND severity=critical", or "any" with no conditions.
func DescribeMatchers(matchers []LabelMatcher) string {
	if len(matchers) == 0 {
		return "any"
	}
	parts := make([]string, 0, len(matchers))
	for _, m := range matchers {
		parts = append(parts, m.String())
	}
	return strings.Join(parts, " AND ")
}

func DecodeMatchers(raw string) []LabelMatcher {
	var out []LabelMatcher
	_ = json.Unmarshal([]byte(raw), &out)
	return out
}

func eventLabels(agent db.Agent, monitor db.Monitor, severity, eventType string) []string {
	labels := append([]string{}, DecodeAgentLabels(agent.Labels)...)
	add := func(k, v string) {
		if strings.TrimSpace(v) != "" {
			labels = append(labels, k+"="+v)
		}
	}
	add("severity", severity)
	add("event", eventType)
	add("monitor_type", monitor.Type)
	add("server", agent.Name)
	add("monitor", monitor.Name)
	return labels
}

func labelSet(labels []string) map[string][]string {
	out := map[string][]string{}
	for _, l := range labels {
		if k, v, ok := strings.Cut(l, "="); ok {
			out[k] = append(out[k], v)
		}
	}
	return out
}

// MatchersMatch is true when every matcher holds for the label set ("=" needs the value present for
// the key, "!=" needs it absent).
func MatchersMatch(matchers []LabelMatcher, labels []string) bool {
	set := labelSet(labels)
	for _, m := range matchers {
		has := false
		for _, v := range set[m.Key] {
			if v == m.Value {
				has = true
			}
		}
		if (m.Op == "=") != has {
			return false
		}
	}
	return true
}

type LabelRouteInput struct {
	Name             string
	Matchers         []LabelMatcher
	ChannelIDs       []string
	ContinueMatching bool
	Enabled          *bool
}

func (s *AlertService) validateLabelRoute(in LabelRouteInput) (LabelRouteInput, error) {
	in.Name = strings.TrimSpace(in.Name)
	if len(in.Name) > 100 || len(in.Matchers) > 10 {
		return in, fmt.Errorf("%w: name or matcher list is too long", ErrLabelRouteInvalid)
	}
	for i, m := range in.Matchers {
		in.Matchers[i].Key, in.Matchers[i].Value = strings.TrimSpace(m.Key), strings.TrimSpace(m.Value)
		if m.Op == "" {
			in.Matchers[i].Op = "="
		}
		if in.Matchers[i].Op != "=" && in.Matchers[i].Op != "!=" {
			return in, fmt.Errorf("%w: operator must be = or !=", ErrLabelRouteInvalid)
		}
		if !labelKeyPattern.MatchString(in.Matchers[i].Key) || !labelValuePattern.MatchString(in.Matchers[i].Value) {
			return in, fmt.Errorf("%w: %q is not a valid label matcher", ErrLabelRouteInvalid, m.Key+m.Op+m.Value)
		}
	}
	if len(in.ChannelIDs) == 0 || len(in.ChannelIDs) > 20 {
		return in, fmt.Errorf("%w: choose between 1 and 20 channels", ErrLabelRouteInvalid)
	}
	var found int64
	s.db.Model(&db.AlertChannel{}).Where("id IN ?", in.ChannelIDs).Count(&found)
	if int(found) != len(uniqueStrings(in.ChannelIDs)) {
		return in, fmt.Errorf("%w: a channel does not exist", ErrLabelRouteInvalid)
	}
	in.ChannelIDs = uniqueStrings(in.ChannelIDs)
	return in, nil
}

func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, v := range in {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

func (s *AlertService) ListLabelRoutes() ([]db.AlertLabelRoute, error) {
	var routes []db.AlertLabelRoute
	err := s.db.Order("position ASC, created_at ASC").Find(&routes).Error
	return routes, err
}

func (s *AlertService) CreateLabelRoute(in LabelRouteInput) (*db.AlertLabelRoute, error) {
	in, err := s.validateLabelRoute(in)
	if err != nil {
		return nil, err
	}
	var count int64
	s.db.Model(&db.AlertLabelRoute{}).Count(&count)
	if count >= maxLabelRoutes {
		return nil, fmt.Errorf("%w: at most %d routes", ErrLabelRouteInvalid, maxLabelRoutes)
	}
	var maxPos int
	s.db.Model(&db.AlertLabelRoute{}).Select("COALESCE(MAX(position), 0)").Scan(&maxPos)
	matchers, _ := json.Marshal(in.Matchers)
	channels, _ := json.Marshal(in.ChannelIDs)
	now := time.Now().UTC()
	route := db.AlertLabelRoute{ID: utils.GenerateID("label_route"), Position: maxPos + 1, Name: in.Name, Enabled: in.Enabled == nil || *in.Enabled, Matchers: string(matchers), ChannelIDs: string(channels),
		ContinueMatching: in.ContinueMatching, CreatedAt: now, UpdatedAt: now}
	return &route, s.db.Create(&route).Error
}

func (s *AlertService) UpdateLabelRoute(id string, in LabelRouteInput) (*db.AlertLabelRoute, error) {
	var route db.AlertLabelRoute
	if err := s.db.Where("id = ?", id).First(&route).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrLabelRouteNotFound
		}
		return nil, err
	}
	in, err := s.validateLabelRoute(in)
	if err != nil {
		return nil, err
	}
	matchers, _ := json.Marshal(in.Matchers)
	channels, _ := json.Marshal(in.ChannelIDs)
	route.Name, route.Matchers, route.ChannelIDs, route.ContinueMatching, route.UpdatedAt = in.Name, string(matchers), string(channels), in.ContinueMatching, time.Now().UTC()
	if in.Enabled != nil {
		route.Enabled = *in.Enabled
	}
	return &route, s.db.Save(&route).Error
}

func (s *AlertService) DeleteLabelRoute(id string) error {
	res := s.db.Where("id = ?", id).Delete(&db.AlertLabelRoute{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrLabelRouteNotFound
	}
	return nil
}

// ReorderLabelRoutes sets positions to the order of ids; ids must be exactly the existing routes.
func (s *AlertService) ReorderLabelRoutes(ids []string) error {
	routes, err := s.ListLabelRoutes()
	if err != nil {
		return err
	}
	existing := make([]string, 0, len(routes))
	for _, r := range routes {
		existing = append(existing, r.ID)
	}
	a, b := append([]string(nil), existing...), append([]string(nil), ids...)
	sort.Strings(a)
	sort.Strings(b)
	if strings.Join(a, ",") != strings.Join(b, ",") {
		return fmt.Errorf("%w: ids must list every route exactly once", ErrLabelRouteInvalid)
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		for i, id := range ids {
			if err := tx.Model(&db.AlertLabelRoute{}).Where("id = ?", id).Update("position", i+1).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// LabelRouteMatch is one evaluated route.
type LabelRouteMatch struct {
	Route   db.AlertLabelRoute
	Number  int
	Matched bool
	// Stopped is true when a matching route without Continue ended the evaluation, so later routes were not tried.
	Skipped bool
}

// EvaluateLabelRoutes tries enabled routes in order; the first match wins unless it continues.
func (s *AlertService) EvaluateLabelRoutes(labels []string) ([]LabelRouteMatch, error) {
	routes, err := s.ListLabelRoutes()
	if err != nil {
		return nil, err
	}
	out := make([]LabelRouteMatch, 0, len(routes))
	stopped := false
	for i, r := range routes {
		m := LabelRouteMatch{Route: r, Number: i + 1}
		switch {
		case stopped:
			m.Skipped = true
		case r.Enabled && MatchersMatch(DecodeMatchers(r.Matchers), labels):
			m.Matched = true
			if !r.ContinueMatching {
				stopped = true
			}
		}
		out = append(out, m)
	}
	return out, nil
}

func (s *AlertService) hasLabelRoutes() bool {
	var n int64
	s.db.Model(&db.AlertLabelRoute{}).Where("enabled = ?", true).Count(&n)
	return n > 0
}

// RuleMatchesEvent reports whether a rule would fire for the event (used by replay).
func RuleMatchesEvent(route db.AlertRoute, event AlertRouteContext) bool {
	matched, _ := routeMatchesEvent(route, event)
	return matched
}

// RuleEvents lists the events a rule reacts to.
func RuleEvents(route db.AlertRoute) []string { return decodeAlertRouteEvents(route.EventTypes) }

// DecodeList reads a stored JSON string list.
func DecodeList(raw string) []string { return decodeStringList(raw) }
