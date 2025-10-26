package registration

import (
	"fmt"

	"orion/agent/internal/config"
	"orion/agent/internal/transport"
	"orion/agent/internal/utils"
)

type Service struct {
	config     *config.Config
	client     *transport.Client
	configPath string
}

func New(cfg *config.Config, configPath string) *Service {
	return &Service{
		config:     cfg,
		client:     transport.NewClient(cfg.CoreURL, ""),
		configPath: configPath,
	}
}

// RegisterIfNeeded checks if the agent is registered and registers if necessary
func (s *Service) RegisterIfNeeded() error {
	// If already registered, no need to register again
	if s.config.IsRegistered() {
		return nil
	}

	uuid, err := utils.GenerateAgentUUID()
	if err != nil {
		return fmt.Errorf("failed to generate agent UUID: %w", err)
	}

	// Get system information
	name, os, arch, err := utils.GetSystemInfo()
	if err != nil {
		return fmt.Errorf("failed to get system info: %w", err)
	}

	// Create registration request
	req := transport.RegistrationRequest{
		MachineId: uuid,
		Name:      name,
		OS:        os,
		Arch:      arch,
	}

	resp, err := s.client.RegisterAgent(req)
	if err != nil {
		return fmt.Errorf("failed to register agent: %w", err)
	}

	s.config.UpdateRegistration(resp.Data.AgentID, resp.Data.Token)

	if err := s.config.Save(s.configPath); err != nil {
		return fmt.Errorf("failed to save updated config: %w", err)
	}

	return nil
}
