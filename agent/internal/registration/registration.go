package registration

import (
	"fmt"
	"time"

	"orion/agent/internal/config"
	"orion/agent/internal/logging"
	"orion/agent/internal/transport"
	"orion/agent/internal/utils"
)

type RegistrationService struct {
	userConfig        *config.UserConfig
	internalState     *config.InternalState
	client            *transport.Client
	userConfigPath    string
	internalStatePath string
}

func New(userConfig *config.UserConfig, userConfigPath string, internalState *config.InternalState, internalStatePath string) *RegistrationService {
	return &RegistrationService{
		userConfig:        userConfig,
		client:            transport.NewClient(userConfig.CoreURL, ""),
		userConfigPath:    userConfigPath,
		internalState:     internalState,
		internalStatePath: internalStatePath,
	}
}

func (s *RegistrationService) RegisterAgentIfNeeded() error {
	if s.internalState.IsRegistered() {
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

	req := transport.AgentRegistrationRequest{
		MachineId: uuid,
		Name:      name,
		OS:        os,
		Arch:      arch,
	}

	resp, err := s.client.RegisterAgent(req)
	if err != nil {
		return fmt.Errorf("failed to register agent: %w", err)
	}

	s.internalState.UpdateRegistration(resp.Data.AgentID, resp.Data.Token, s.userConfig.CoreURL)
	s.client.SetAuthToken(resp.Data.Token)

	if err := s.internalState.Save(s.internalStatePath); err != nil {
		return fmt.Errorf("failed to save updated config: %w", err)
	}

	if err := s.RegisterAgentMonitorsIfNeeded(); err != nil {
		return fmt.Errorf("failed to register monitors: %w", err)
	}

	return nil
}

func (s *RegistrationService) RegisterAgentMonitorsIfNeeded() error {
	s.client.SetAuthToken(s.internalState.Token)

	configMonitors := make(map[string]config.UserMonitor)
	for _, m := range s.userConfig.Monitors {
		configMonitors[m.Name] = m
	}

	stateMonitors := buildStateMonitorMap(s.internalState.Monitors)

	logging.Infof("stateMonitors: %v", stateMonitors)

	var updatedState []config.InternalStateMonitor

	// register new monitors
	for name, monitor := range configMonitors {
		if _, exists := stateMonitors[name]; exists {
			// already registered — keep it
			updatedState = append(updatedState, stateMonitors[name])
			continue
		}

		logging.Infof("Registering monitor %q", name)

		req := transport.MonitorRegistrationRequest{
			AgentID:     s.internalState.AgentID,
			Name:        monitor.Name,
			Description: monitor.Description,
			Type:        string(monitor.Type),
			LastChecked: time.Now(),
		}

		resp, err := s.client.RegisterMonitor(req)
		if err != nil {
			return fmt.Errorf("failed to register monitor %q: %w", name, err)
		}

		updatedState = append(updatedState, config.InternalStateMonitor{
			ID:          resp.Data.MonitorID,
			Name:        name,
			Status:      "running",
			LastChecked: time.Now(),
		})
	}

	// unregister removed monitors
	for name, stateMonitor := range stateMonitors {
		if _, exists := configMonitors[name]; exists {
			continue
		}

		logging.Infof("Unregistering monitor %q", name)

		req := transport.UnRegisterMonitorRequest{
			AgentID:   s.internalState.AgentID,
			MonitorID: stateMonitor.ID,
		}

		_, err := s.client.UnregisterMonitor(req)
		if err != nil {
			return fmt.Errorf("failed to unregister monitor %q: %w", name, err)
		}
	}

	s.internalState.UpdateMonitors(updatedState)

	if err := s.internalState.Save(s.internalStatePath); err != nil {
		return fmt.Errorf("failed to save updated config: %w", err)
	}

	return nil
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
