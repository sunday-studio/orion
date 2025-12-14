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
			URL:            userMonitorConfig.HTTP.URL,
			Timeout:        timeout,
			ExpectedStatus: 200,
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
		result := RunInternalServiceMonitor(InternalServiceMonitorConfig{
			Ping: PingConfig{
				URL:     userMonitorConfig.InternalService.Ping.URL,
				Timeout: defaultTimeout,
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
	}
	return &MonitorResult{
		Status:    "down",
		Timestamp: time.Now().UTC(),
		Metrics:   &map[string]interface{}{},
		Error:     &MonitorError{Message: "unsupported monitor type"},
	}, errors.New("unsupported monitor type")
}
