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

	if err := s.RegisterAgentApplicationsIfNeeded(); err != nil {
		return fmt.Errorf("failed to register agent applications: %w", err)
	}

	return nil
}

func (s *RegistrationService) RegisterAgentApplicationsIfNeeded() error {
	if len(s.userConfig.Applications) == 0 {
		return nil
	}

	var applications []config.InternalStateApplication

	for _, app := range s.userConfig.Applications {
		req := transport.ApplicationRegistrationRequest{
			AgentID:     s.internalState.AgentID,
			Name:        app.Name,
			Description: app.Description,
			Type:        string(app.Type),
			LastChecked: time.Now(),
		}

		resp, err := s.client.RegisterApplication(req)
		if err != nil {
			return fmt.Errorf("failed to register application: %w", err)
		}

		fmt.Println("resp ->", resp)

		applications = append(applications, config.InternalStateApplication{
			ID:          resp.Data.ApplicationID,
			Name:        app.Name,
			Status:      "running",
			LastChecked: time.Now(),
		})
	}

	s.internalState.UpdateApplications(applications)

	if err := s.internalState.Save(s.internalStatePath); err != nil {
		return fmt.Errorf("failed to save updated config: %w", err)
	}

	return nil
}
