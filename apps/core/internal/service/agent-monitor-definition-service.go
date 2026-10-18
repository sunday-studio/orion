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
	diskPathPattern       = regexp.MustCompile(`^/[A-Za-z0-9_./ -]{0,200}$`)
)

const maxDefinitionsPerAgent = 50

type AgentMonitorDefinitionInput struct {
	Name            string
	CheckType       string
	Params          map[string]any
	IntervalSeconds int
	FailAfter       int
}

// AgentMonitorDefinitionView adds the live state: waiting until the agent reports under that name.
type AgentMonitorDefinitionView struct {
	db.AgentMonitorDefinition
	State        string // waiting | reporting
	MonitorID    string
	FirstReport  *time.Time
	ParamsParsed map[string]any
}

func validateDefinition(in AgentMonitorDefinitionInput) (AgentMonitorDefinitionInput, error) {
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
	default:
		return in, fmt.Errorf("%w: check_type must be disk, memory, process or port", ErrDefinitionInvalid)
	}
	in.Params = params
	return in, nil
}

func (s *AgentService) CreateMonitorDefinition(agentID string, in AgentMonitorDefinitionInput, actor string) (*db.AgentMonitorDefinition, error) {
	in, err := validateDefinition(in)
	if err != nil {
		return nil, err
	}
	var agent db.Agent
	if err := s.db.Where("id = ? AND (deleted_at IS NULL OR deleted_at = ?)", agentID, time.Time{}).First(&agent).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAgentNotFound
		}
		return nil, err
	}
	var count int64
	s.db.Model(&db.AgentMonitorDefinition{}).Where("agent_id = ?", agentID).Count(&count)
	if count >= maxDefinitionsPerAgent {
		return nil, fmt.Errorf("%w: at most %d per server", ErrDefinitionInvalid, maxDefinitionsPerAgent)
	}
	raw, _ := json.Marshal(in.Params)
	def := db.AgentMonitorDefinition{ID: utils.GenerateID("agent_monitor_def"), AgentID: agentID, Name: in.Name, CheckType: in.CheckType, Params: string(raw),
		IntervalSeconds: in.IntervalSeconds, FailAfter: in.FailAfter, CreatedBy: actor, CreatedAt: time.Now().UTC()}
	if err := s.db.Create(&def).Error; err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return nil, ErrDefinitionExists
		}
		return nil, err
	}
	return &def, nil
}

func (s *AgentService) DeleteMonitorDefinition(agentID, definitionID string) error {
	res := s.db.Where("id = ? AND agent_id = ?", definitionID, agentID).Delete(&db.AgentMonitorDefinition{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrDefinitionNotFound
	}
	return nil
}

// ListMonitorDefinitions returns the agent's definitions with their live state.
func (s *AgentService) ListMonitorDefinitions(agentID string) ([]AgentMonitorDefinitionView, error) {
	var defs []db.AgentMonitorDefinition
	if err := s.db.Where("agent_id = ?", agentID).Order("created_at ASC").Find(&defs).Error; err != nil {
		return nil, err
	}
	views := make([]AgentMonitorDefinitionView, 0, len(defs))
	for _, d := range defs {
		v := AgentMonitorDefinitionView{AgentMonitorDefinition: d, State: "waiting", ParamsParsed: map[string]any{}}
		_ = json.Unmarshal([]byte(d.Params), &v.ParamsParsed)
		var m db.Monitor
		if s.db.Where("agent_id = ? AND name = ? AND lifecycle <> ?", agentID, d.Name, "deleted").Limit(1).Find(&m).RowsAffected > 0 {
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
