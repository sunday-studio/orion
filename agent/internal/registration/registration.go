package registration

import (
	"fmt"

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

	s.internalState.UpdateRegistration(resp.Data.AgentID, resp.Data.Token)

	// if err := s.userConfig.Save(s.userConfigPath); err != nil {
	// 	return fmt.Errorf("failed to save updated config: %w", err)
	// }

	return nil
}

func (s *RegistrationService) RegisterAgentApplicationsIfNeeded() error {
	if len(s.userConfig.Applications) == 0 {
		return nil
	}

	for _, app := range s.userConfig.Applications {

		fmt.Println("app name ->", app.Name)
		// req := transport.ApplicationRegistrationRequest{
		// 	AgentID: s.config.AgentID,
		// 	AppID:   app.ID,
		// }
	}

	return nil
}
