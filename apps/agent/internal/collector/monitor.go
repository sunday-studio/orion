package collector

import (
	"errors"
	"orion/agent/internal/config"
	"time"
)

type MonitorResult struct {
	Status    string        `json:"status"`
	Timestamp time.Time     `json:"timestamp"`
	Metrics   interface{}   `json:"metrics,omitempty"`
	Error     *MonitorError `json:"error,omitempty"`
}

func CollectMonitorReport(internalMonitor config.InternalStateMonitor, userMonitorConfig config.UserMonitor) (*MonitorResult, error) {
	defaultTimeout := 10 * time.Second

	switch userMonitorConfig.Type {
	case config.UserMonitorTypeHTTPHealthcheck:
		timeout := defaultTimeout
		if userMonitorConfig.HTTP.Timeout != "" {
			if d, err := time.ParseDuration(userMonitorConfig.HTTP.Timeout); err == nil {
				timeout = d
			}
		}
		httpResult, err := RunHTTPMonitor(HTTPMonitorConfig{
			URL:               userMonitorConfig.HTTP.URL,
			Timeout:           timeout,
			ExpectedStatus:    userMonitorConfig.HTTP.ExpectedStatus,
			ExpectedBody:      userMonitorConfig.HTTP.ExpectedBody,
			ExpectedBodyRegex: userMonitorConfig.HTTP.ExpectedBodyRegex,
		})
		if err != nil {
			return &MonitorResult{
				Status:    "down",
				Timestamp: time.Now().UTC(),
				Metrics:   &map[string]interface{}{},
				Error:     &MonitorError{Message: err.Error()},
			}, err
		} else {
			return &MonitorResult{
				Status:    httpResult.Status,
				Timestamp: httpResult.Timestamp,
				Metrics:   &httpResult.Metrics,
			}, nil
		}

	case config.UserMonitorInternalService:
		timeout := defaultTimeout
		if userMonitorConfig.InternalService.Ping.Timeout != "" {
			if d, err := time.ParseDuration(userMonitorConfig.InternalService.Ping.Timeout); err == nil {
				timeout = d
			}
		}
		result := RunInternalServiceMonitor(InternalServiceMonitorConfig{
			Ping: PingConfig{
				URL:     userMonitorConfig.InternalService.Ping.URL,
				Timeout: timeout,
			},
			Process: PortProcessConfig{
				Port: userMonitorConfig.InternalService.Process.Port,
			},
		})
		if result.Error != nil {
			return &MonitorResult{
				Status:    result.Status,
				Timestamp: result.Timestamp,
				Metrics:   &result.Metrics,
				Error:     result.Error,
			}, errors.New(result.Error.Message)
		}
		return &MonitorResult{
			Status:    result.Status,
			Timestamp: result.Timestamp,
			Metrics:   &result.Metrics,
			Error:     result.Error,
		}, nil
	case config.UserMonitorTypeCommand:
		timeout := defaultTimeout
		if userMonitorConfig.Command.Timeout != "" {
			if d, err := time.ParseDuration(userMonitorConfig.Command.Timeout); err == nil {
				timeout = d
			}
		}
		result := RunCommandMonitor(CommandMonitorConfig{
			Command: userMonitorConfig.Command.Command,
			Args:    userMonitorConfig.Command.Args,
			Timeout: timeout,
		})
		if result.Error != nil {
			return &MonitorResult{
				Status:    result.Status,
				Timestamp: result.Timestamp,
				Metrics:   &result.Metrics,
				Error:     result.Error,
			}, errors.New(result.Error.Message)
		}
		return &MonitorResult{
			Status:    result.Status,
			Timestamp: result.Timestamp,
			Metrics:   &result.Metrics,
			Error:     result.Error,
		}, nil
	case config.UserMonitorTypeWebsite:
		result := RunWebsiteMonitor(*userMonitorConfig.Website)
		if result.Error != nil {
			return &MonitorResult{
				Status:    result.Status,
				Timestamp: result.Timestamp,
				Metrics:   &result.Metrics,
				Error:     result.Error,
			}, errors.New(result.Error.Message)
		}
		return &MonitorResult{
			Status:    result.Status,
			Timestamp: result.Timestamp,
			Metrics:   &result.Metrics,
			Error:     result.Error,
		}, nil
	case config.UserMonitorTypePM2:
		result := RunPM2Monitor(PM2MonitorConfig{
			AppName: userMonitorConfig.PM2.AppName,
		})
		if result.Error != nil {
			return &MonitorResult{
				Status:    result.Status,
				Timestamp: result.Timestamp,
				Metrics:   &result.Metrics,
				Error:     result.Error,
			}, errors.New(result.Error.Message)
		}
		return &MonitorResult{
			Status:    result.Status,
			Timestamp: result.Timestamp,
			Metrics:   &result.Metrics,
			Error:     result.Error,
		}, nil
	case config.UserMonitorTypeTCP:
		timeout := defaultTimeout
		if userMonitorConfig.TCP.Timeout != "" {
			if d, err := time.ParseDuration(userMonitorConfig.TCP.Timeout); err == nil {
				timeout = d
			}
		}
		result := RunTCPMonitor(TCPMonitorConfig{
			Host:    userMonitorConfig.TCP.Host,
			Port:    userMonitorConfig.TCP.Port,
			Timeout: timeout,
		})
		if result.Error != nil {
			return &MonitorResult{
				Status:    result.Status,
				Timestamp: result.Timestamp,
				Metrics:   &result.Metrics,
				Error:     result.Error,
			}, errors.New(result.Error.Message)
		}
		return &MonitorResult{
			Status:    result.Status,
			Timestamp: result.Timestamp,
			Metrics:   &result.Metrics,
			Error:     result.Error,
		}, nil
	case config.UserMonitorTypeResource:
		result := RunResourceThresholdMonitor(ResourceThresholdConfig{
			MaxCPUPercent:    userMonitorConfig.Resource.MaxCPUPercent,
			MaxMemoryPercent: userMonitorConfig.Resource.MaxMemoryPercent,
			MaxDiskPercent:   userMonitorConfig.Resource.MaxDiskPercent,
			MaxLoad1:         userMonitorConfig.Resource.MaxLoad1,
		})
		if result.Error != nil {
			return &MonitorResult{
				Status:    result.Status,
				Timestamp: result.Timestamp,
				Metrics:   &result.Metrics,
				Error:     result.Error,
			}, errors.New(result.Error.Message)
		}
		return &MonitorResult{
			Status:    result.Status,
			Timestamp: result.Timestamp,
			Metrics:   &result.Metrics,
			Error:     result.Error,
		}, nil
	case config.UserMonitorTypeDocker:
		result := RunDockerContainerMonitor(DockerContainerConfig{
			Name: userMonitorConfig.Docker.Name,
		})
		if result.Error != nil {
			return &MonitorResult{
				Status:    result.Status,
				Timestamp: result.Timestamp,
				Metrics:   &result.Metrics,
				Error:     result.Error,
			}, errors.New(result.Error.Message)
		}
		return &MonitorResult{
			Status:    result.Status,
			Timestamp: result.Timestamp,
			Metrics:   &result.Metrics,
			Error:     result.Error,
		}, nil
	case config.UserMonitorTypeSystemd:
		result := RunSystemdServiceMonitor(SystemdServiceConfig{
			Name: userMonitorConfig.Systemd.Name,
		})
		if result.Error != nil {
			return &MonitorResult{
				Status:    result.Status,
				Timestamp: result.Timestamp,
				Metrics:   &result.Metrics,
				Error:     result.Error,
			}, errors.New(result.Error.Message)
		}
		return &MonitorResult{
			Status:    result.Status,
			Timestamp: result.Timestamp,
			Metrics:   &result.Metrics,
			Error:     result.Error,
		}, nil
	}
	return &MonitorResult{
		Status:    "down",
		Timestamp: time.Now().UTC(),
		Metrics:   &map[string]interface{}{},
		Error:     &MonitorError{Message: "unsupported monitor type"},
	}, errors.New("unsupported monitor type")
}
