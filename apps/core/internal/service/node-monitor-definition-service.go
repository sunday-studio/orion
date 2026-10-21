package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"regexp"
	"strings"
	"time"

	"gorm.io/gorm"
)

var (
	ErrDefinitionInvalid  = errors.New("invalid monitor definition")
	ErrDefinitionNotFound = errors.New("monitor definition not found")
	ErrDefinitionExists   = errors.New("this server already has a monitor with that name")
	processNamePattern    = regexp.MustCompile(`^[A-Za-z0-9._@:-]{1,100}$`)
	containerNamePattern  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$`)
	pm2NamePattern        = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.@:-]{0,99}$`)
	diskPathPattern       = regexp.MustCompile(`^/[A-Za-z0-9_./ -]{0,200}$`)
)

const maxDefinitionsPerNode = 50

type NodeMonitorDefinitionInput struct {
	Name            string
	CheckType       string
	Params          map[string]any
	IntervalSeconds int
	FailAfter       int
}

// NodeMonitorDefinitionView adds the live state: waiting until the node reports under that name.
type NodeMonitorDefinitionView struct {
	db.NodeMonitorDefinition
	State         string // waiting | applied | failed | reporting
	FailureReason string // why the node could not run it, in its own words
	MonitorID     string
	FirstReport   *time.Time
	ParamsParsed  map[string]any
}

func validateDefinition(in NodeMonitorDefinitionInput) (NodeMonitorDefinitionInput, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || len(in.Name) > 100 {
		return in, fmt.Errorf("%w: name is required (max 100 characters)", ErrDefinitionInvalid)
	}
	if in.IntervalSeconds == 0 {
		in.IntervalSeconds = 30
	}
	if in.IntervalSeconds < 10 || in.IntervalSeconds > 86400 {
		return in, fmt.Errorf("%w: interval must be between 10 seconds and 24 hours", ErrDefinitionInvalid)
	}
	if in.FailAfter == 0 {
		in.FailAfter = 3
	}
	if in.FailAfter < 1 || in.FailAfter > 20 {
		return in, fmt.Errorf("%w: fail_after must be between 1 and 20", ErrDefinitionInvalid)
	}
	params := map[string]any{}
	number := func(key string, def, min, max float64) (float64, error) {
		v, ok := in.Params[key]
		if !ok {
			return def, nil
		}
		f, isNum := v.(float64)
		if !isNum || f < min || f > max {
			return 0, fmt.Errorf("%w: %s must be a number between %v and %v", ErrDefinitionInvalid, key, min, max)
		}
		return f, nil
	}
	switch in.CheckType {
	case "disk":
		path := "/"
		if v, ok := in.Params["path"].(string); ok && v != "" {
			path = v
		}
		if !diskPathPattern.MatchString(path) || strings.Contains(path, "..") {
			return in, fmt.Errorf("%w: path must be an absolute path", ErrDefinitionInvalid)
		}
		t, err := number("threshold_percent", 90, 1, 100)
		if err != nil {
			return in, err
		}
		params["path"], params["threshold_percent"] = path, t
	case "memory":
		t, err := number("threshold_percent", 90, 1, 100)
		if err != nil {
			return in, err
		}
		params["threshold_percent"] = t
	case "process":
		name, _ := in.Params["process_name"].(string)
		if !processNamePattern.MatchString(name) {
			return in, fmt.Errorf("%w: process_name is required (letters, digits and . _ @ : -)", ErrDefinitionInvalid)
		}
		params["process_name"] = name
	case "port":
		p, err := number("port", 0, 1, 65535)
		if err != nil || p == 0 {
			return in, fmt.Errorf("%w: port must be between 1 and 65535", ErrDefinitionInvalid)
		}
		params["port"] = p
	case "systemd":
		unit, _ := in.Params["unit"].(string)
		unit = strings.TrimSuffix(strings.TrimSpace(unit), ".service")
		if !processNamePattern.MatchString(unit) {
			return in, fmt.Errorf("%w: unit is required (letters, digits and . _ @ : -)", ErrDefinitionInvalid)
		}
		params["unit"] = unit
	case "docker":
		name, _ := in.Params["container"].(string)
		if !containerNamePattern.MatchString(strings.TrimSpace(name)) {
			return in, fmt.Errorf("%w: container is required (a container name)", ErrDefinitionInvalid)
		}
		params["container"] = strings.TrimSpace(name)
	case "pm2":
		app, _ := in.Params["app"].(string)
		if !pm2NamePattern.MatchString(strings.TrimSpace(app)) {
			return in, fmt.Errorf("%w: app is required (a pm2 app name)", ErrDefinitionInvalid)
		}
		params["app"] = strings.TrimSpace(app)
	default:
		return in, fmt.Errorf("%w: check_type must be disk, memory, process, port, systemd, docker or pm2", ErrDefinitionInvalid)
	}
	in.Params = params
	return in, nil
}

func (s *NodeService) CreateMonitorDefinition(nodeID string, in NodeMonitorDefinitionInput, actor string) (*db.NodeMonitorDefinition, error) {
	in, err := validateDefinition(in)
	if err != nil {
		return nil, err
	}
	var node db.Node
	if err := s.db.Where("id = ? AND (deleted_at IS NULL OR deleted_at = ?)", nodeID, time.Time{}).First(&node).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNodeNotFound
		}
		return nil, err
	}
	var count int64
	s.db.Model(&db.NodeMonitorDefinition{}).Where("node_id = ?", nodeID).Count(&count)
	if count >= maxDefinitionsPerNode {
		return nil, fmt.Errorf("%w: at most %d per server", ErrDefinitionInvalid, maxDefinitionsPerNode)
	}
	raw, _ := json.Marshal(in.Params)
	def := db.NodeMonitorDefinition{ID: utils.GenerateID("node_monitor_def"), NodeID: nodeID, Name: in.Name, CheckType: in.CheckType, Params: string(raw),
		IntervalSeconds: in.IntervalSeconds, FailAfter: in.FailAfter, CreatedBy: actor, CreatedAt: time.Now().UTC()}
	if err := s.db.Create(&def).Error; err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return nil, ErrDefinitionExists
		}
		return nil, err
	}
	return &def, nil
}

func (s *NodeService) DeleteMonitorDefinition(nodeID, definitionID string) error {
	res := s.db.Where("id = ? AND node_id = ?", definitionID, nodeID).Delete(&db.NodeMonitorDefinition{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrDefinitionNotFound
	}
	return nil
}

// ListMonitorDefinitions returns the node's definitions with their live state.
func (s *NodeService) ListMonitorDefinitions(nodeID string) ([]NodeMonitorDefinitionView, error) {
	var defs []db.NodeMonitorDefinition
	if err := s.db.Where("node_id = ?", nodeID).Order("created_at ASC").Find(&defs).Error; err != nil {
		return nil, err
	}
	views := make([]NodeMonitorDefinitionView, 0, len(defs))
	for _, d := range defs {
		v := NodeMonitorDefinitionView{NodeMonitorDefinition: d, State: "waiting", ParamsParsed: map[string]any{}}
		if d.FailureReason != "" {
			v.State, v.FailureReason = "failed", d.FailureReason
		}
		_ = json.Unmarshal([]byte(d.Params), &v.ParamsParsed)
		var m db.Monitor
		if s.db.Where("node_id = ? AND name = ? AND lifecycle <> ?", nodeID, d.Name, "deleted").Limit(1).Find(&m).RowsAffected > 0 {
			v.MonitorID = m.ID
			var r db.MonitorReport
			if s.db.Where("monitor_id = ?", m.ID).Order("created_at ASC").Limit(1).Find(&r).RowsAffected > 0 {
				v.State = "reporting"
				t := r.CreatedAt
				v.FirstReport = &t
			}
		}
		views = append(views, v)
	}
	return views, nil
}

// MarkDefinitionsDelivered notes that the node has fetched its definitions.
func (s *NodeService) MarkDefinitionsDelivered(nodeID string) {
	now := time.Now().UTC()
	s.db.Model(&db.NodeMonitorDefinition{}).Where("node_id = ? AND delivered_at IS NULL", nodeID).Update("delivered_at", now)
}

// DefinitionResult is what the node says about trying to run one definition.
type DefinitionResult struct {
	DefinitionID string `json:"definition_id"`
	OK           bool   `json:"ok"`
	Reason       string `json:"reason,omitempty"`
}

// RecordDefinitionResults stores why a definition could not run, or clears it when it can.
func (s *NodeService) RecordDefinitionResults(nodeID string, results []DefinitionResult) error {
	if len(results) > maxDefinitionsPerNode*2 {
		return fmt.Errorf("%w: too many results", ErrDefinitionInvalid)
	}
	for _, r := range results {
		reason := ""
		if !r.OK {
			reason = clipText(r.Reason)
			if reason == "" {
				reason = "The node could not run this check."
			}
		}
		if err := s.db.Model(&db.NodeMonitorDefinition{}).Where("id = ? AND node_id = ?", r.DefinitionID, nodeID).Update("failure_reason", reason).Error; err != nil {
			return err
		}
	}
	return nil
}
