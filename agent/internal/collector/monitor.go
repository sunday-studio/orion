package collector

import (
	"fmt"

	"orion/agent/internal/config"
	"orion/agent/internal/utils"
	"time"

	httpMonitor "orion/agent/internal/collector/http_monitor"
	internalServiceMonitor "orion/agent/internal/collector/internal_service_monitor"
)

func CollectMonitorReport(internalMonitor config.InternalStateMonitor, userMonitorConfig config.UserMonitor) error {
	utils.PrettyPrint(internalMonitor)
	utils.PrettyPrint(userMonitorConfig)

	defaultTimeout := 10 * time.Second

	switch userMonitorConfig.Type {
	case config.UserMonitorTypeHTTPHealthcheck:
		return httpMonitor.RunHTTPMonitor(httpMonitor.HTTPMonitorConfig{
			URL:            userMonitorConfig.HTTP.URL,
			Timeout:        userMonitorConfig.HTTP.Timeout,
			ExpectedStatus: 200,
		})
	case config.UserMonitorInternalService:
		return internalServiceMonitor.RunInternalServiceMonitor(internalServiceMonitor.InternalServiceMonitorConfig{
			Ping: internalServiceMonitor.PingConfig{
				URL:     userMonitorConfig.InternalService.Ping.URL,
				Timeout: defaultTimeout,
			},
		})
	}
	return nil
}

func CollectNginxMonitorReport(internalMonitor config.InternalStateMonitor, userMonitorConfig config.UserMonitor) error {
	fmt.Println("collecting nginx monitor report")
	return nil
}
