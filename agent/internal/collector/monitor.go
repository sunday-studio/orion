package collector

import (
	"errors"
	"orion/agent/internal/config"
	"orion/agent/internal/utils"
	"time"
)

func CollectMonitorReport(internalMonitor config.InternalStateMonitor, userMonitorConfig config.UserMonitor) error {
	utils.PrettyPrint(internalMonitor)
	utils.PrettyPrint(userMonitorConfig)

	defaultTimeout := 10 * time.Second

	switch userMonitorConfig.Type {
	case config.UserMonitorTypeHTTPHealthcheck:
		timeout := defaultTimeout
		if userMonitorConfig.HTTP.Timeout != "" {
			if d, err := time.ParseDuration(userMonitorConfig.HTTP.Timeout); err == nil {
				timeout = d
			}
		}
		_, err := RunHTTPMonitor(HTTPMonitorConfig{
			URL:            userMonitorConfig.HTTP.URL,
			Timeout:        timeout,
			ExpectedStatus: 200,
		})
		return err
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
			return errors.New(result.Error.Message)
		}
		return nil
	case config.UserMonitorTypeWebsite:
		result := RunWebsiteMonitor(*userMonitorConfig.Website)
		if result.Error != nil {
			return errors.New(result.Error.Message)
		}
		return nil
	}
	return nil
}
