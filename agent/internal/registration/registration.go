package registration

import (
	"fmt"
	"time"

	"orion/agent/internal/config"
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
		return fmt.Errorf("failed to register agent applications: %w", err)
	}

	return nil
}

func (s *RegistrationService) RegisterAgentMonitorsIfNeeded() error {
	if len(s.userConfig.Monitors) == 0 {
		return nil
	}

	var monitors []config.InternalStateMonitor

	for _, monitor := range s.userConfig.Monitors {
		req := transport.MonitorRegistrationRequest{
			AgentID:     s.internalState.AgentID,
			Name:        monitor.Name,
			Description: monitor.Description,
			Type:        string(monitor.Type),
			LastChecked: time.Now(),
		}

		resp, err := s.client.RegisterMonitor(req)
		if err != nil {
			return fmt.Errorf("failed to register monitor: %w", err)
		}

		monitors = append(monitors, config.InternalStateMonitor{
			ID:          resp.Data.MonitorID,
			Name:        monitor.Name,
			Status:      "running",
			LastChecked: time.Now(),
		})
	}

	s.internalState.UpdateMonitors(monitors)

	if err := s.internalState.Save(s.internalStatePath); err != nil {
		return fmt.Errorf("failed to save updated config: %w", err)
	}

	return nil
}
