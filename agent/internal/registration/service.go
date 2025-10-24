package registration

import (
	"fmt"
	"strconv"

	"orion/agent/internal/config"
	"orion/agent/internal/transport"
	"orion/agent/internal/utils"
)

// Service handles agent registration with the Orion Core
type Service struct {
	config   *config.Config
	client   *transport.Client
	configPath string
}

// New creates a new registration service
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

	// Generate UUID for this agent
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
		UUID: uuid,
		Name: name,
		OS:   os,
		Arch: arch,
	}

	// Send registration request
	resp, err := s.client.RegisterAgent(req)
	if err != nil {
		return fmt.Errorf("failed to register agent: %w", err)
	}

	// Update config with registration data
	agentID := strconv.Itoa(resp.Data.AgentID)
	s.config.UpdateRegistration(agentID, resp.Data.Token)

	// Save updated config
	if err := s.config.Save(s.configPath); err != nil {
		return fmt.Errorf("failed to save updated config: %w", err)
	}

	return nil
}
