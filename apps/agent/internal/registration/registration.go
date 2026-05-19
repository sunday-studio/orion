package registration

import (
	"encoding/json"
	"fmt"
	"time"

	"orion/agent/internal/config"
	"orion/agent/internal/logging"
	"orion/agent/internal/state"
	"orion/agent/internal/transport"
	"orion/agent/internal/utils"
)

type RegistrationService struct {
	userConfig     *config.UserConfig
	stateStore     *state.Store
	client         *transport.Client
	userConfigPath string
}

func New(userConfig *config.UserConfig, userConfigPath string, stateStore *state.Store) *RegistrationService {
	return &RegistrationService{
		userConfig:     userConfig,
		client:         transport.NewClient(userConfig.CoreURL, ""),
		userConfigPath: userConfigPath,
		stateStore:     stateStore,
	}
}

func (s *RegistrationService) RegisterAgentIfNeeded() error {
	internalState, err := s.stateStore.Get()
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}
	logging.Debugf("registration state loaded: registered=%t agent_id=%s core_url=%s monitors=%d", internalState.IsRegistered(), internalState.AgentID, internalState.CoreURL, len(internalState.Monitors))
	if internalState.IsRegistered() {
		logging.Debugf("existing agent registration found; reconciling monitors")
		if err := s.RegisterAgentMonitorsIfNeeded(); err != nil {
			return fmt.Errorf("failed to register monitors: %w", err)
		}
		return nil
	}

	uuid, err := utils.GenerateAgentUUID()
	if err != nil {
		return fmt.Errorf("failed to generate agent UUID: %w", err)
	}

	name, os, arch, err := utils.GetSystemInfo()
	if err != nil {
		return fmt.Errorf("failed to get system info: %w", err)
	}
	logging.Debugf("registering agent: machine_id=%s name=%s os=%s arch=%s interval=%s", uuid, name, os, arch, s.userConfig.Interval)

	req := transport.AgentRegistrationRequest{
		MachineId:                uuid,
		Name:                     name,
		OS:                       os,
		Arch:                     arch,
		ReportingIntervalSeconds: intervalSeconds(s.userConfig.Interval),
	}
	if s.userConfig.Meta != nil && len(s.userConfig.Meta) > 0 {
		if b, err := json.Marshal(s.userConfig.Meta); err == nil {
			req.Meta = string(b)
		}
	}

	resp, err := s.client.RegisterAgent(req)
	if err != nil {
		return fmt.Errorf("failed to register agent: %w", err)
	}

	if err := s.stateStore.UpdateRegistration(resp.Data.AgentID, resp.Data.Token, s.userConfig.CoreURL); err != nil {
		return fmt.Errorf("failed to save registration state: %w", err)
	}
	s.client.SetAuthToken(resp.Data.Token)

	if err := s.RegisterAgentMonitorsIfNeeded(); err != nil {
		return fmt.Errorf("failed to register monitors: %w", err)
	}

	return nil
}

func (s *RegistrationService) RegisterAgentMonitorsIfNeeded() error {
	internalState, err := s.stateStore.Get()
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}
	s.client.SetAuthToken(internalState.Token)

	configMonitors := make(map[string]config.UserMonitor)
	for _, m := range s.userConfig.Monitors {
		configMonitors[m.Name] = m
	}

	stateMonitors := buildStateMonitorMap(internalState.Monitors)
	logging.Debugf("monitor reconciliation started: configured=%d state=%d", len(configMonitors), len(stateMonitors))

	var updatedState []config.InternalStateMonitor

	// register or reconcile all configured monitors. Core updates interval/meta
	// for already-active monitors and revives deleted monitors by name.
	for name, monitor := range configMonitors {
		logging.Infof("Reconciling monitor %q", name)

		req := transport.MonitorRegistrationRequest{
			AgentID:                  internalState.AgentID,
			Name:                     monitor.Name,
			Description:              monitor.Description,
			Type:                     string(monitor.Type),
			LastChecked:              time.Now(),
			ReportingIntervalSeconds: intervalSeconds(monitor.Interval),
		}
		if monitor.Meta != nil && len(monitor.Meta) > 0 {
			if b, err := json.Marshal(monitor.Meta); err == nil {
				req.Meta = string(b)
			}
		}

		resp, err := s.client.RegisterMonitor(req)
		if err != nil {
			return fmt.Errorf("failed to reconcile monitor %q: %w", name, err)
		}
		logging.Debugf("monitor reconciled: name=%s id=%s type=%s interval_seconds=%d", name, resp.Data.MonitorID, monitor.Type, req.ReportingIntervalSeconds)

		lastChecked := time.Now()
		status := "running"
		if stateMonitor, exists := stateMonitors[name]; exists {
			lastChecked = stateMonitor.LastChecked
			if stateMonitor.Status != "" {
				status = stateMonitor.Status
			}
		}

		updatedState = append(updatedState, config.InternalStateMonitor{
			ID:          resp.Data.MonitorID,
			Name:        name,
			Status:      status,
			LastChecked: lastChecked,
		})
	}

	// unregister removed monitors
	for name, stateMonitor := range stateMonitors {
		if _, exists := configMonitors[name]; exists {
			continue
		}

		logging.Infof("Unregistering monitor %q", name)

		req := transport.UnRegisterMonitorRequest{
			AgentID:   internalState.AgentID,
			MonitorID: stateMonitor.ID,
		}

		_, err := s.client.UnregisterMonitor(req)
		if err != nil {
			return fmt.Errorf("failed to unregister monitor %q: %w", name, err)
		}
		logging.Debugf("monitor unregistered: name=%s id=%s", name, stateMonitor.ID)
	}

	if err := s.stateStore.ReplaceMonitors(updatedState); err != nil {
		return fmt.Errorf("failed to save monitor state: %w", err)
	}
	logging.Debugf("monitor reconciliation complete: saved_mappings=%d", len(updatedState))

	return nil
}

func intervalSeconds(value string) int {
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return 60
	}
	return int(duration.Seconds())
}

func buildStateMonitorMap(monitors []config.InternalStateMonitor) map[string]config.InternalStateMonitor {
	if monitors == nil {
		return map[string]config.InternalStateMonitor{}
	}
	m := make(map[string]config.InternalStateMonitor)
	for _, monitor := range monitors {
		m[monitor.Name] = monitor
	}
	return m
}
