package collector

import (
	"fmt"
	"net"
	"time"
)

type TCPMonitorConfig struct {
	Host    string
	Port    int
	Timeout time.Duration
}

type TCPMonitorResult struct {
	Status    string                 `json:"status"`
	Timestamp time.Time              `json:"timestamp"`
	Metrics   map[string]interface{} `json:"metrics,omitempty"`
	Error     *MonitorError          `json:"error,omitempty"`
}

type tcpDialFunc func(network string, address string, timeout time.Duration) (net.Conn, error)

func RunTCPMonitor(cfg TCPMonitorConfig) *TCPMonitorResult {
	dialer := &net.Dialer{Timeout: cfg.Timeout}
	return runTCPMonitorWithDialer(cfg, func(network string, address string, timeout time.Duration) (net.Conn, error) {
		return dialer.Dial(network, address)
	})
}

func runTCPMonitorWithDialer(cfg TCPMonitorConfig, dial tcpDialFunc) *TCPMonitorResult {
	start := time.Now()
	address := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)

	conn, err := dial("tcp", address, cfg.Timeout)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return &TCPMonitorResult{
			Status:    "down",
			Timestamp: time.Now().UTC(),
			Metrics: map[string]interface{}{
				"address":        address,
				"latency_ms":     latency,
				"failure_reason": "tcp connection failed",
			},
			Error: &MonitorError{Message: err.Error()},
		}
	}
	defer conn.Close()

	return &TCPMonitorResult{
		Status:    "up",
		Timestamp: time.Now().UTC(),
		Metrics: map[string]interface{}{
			"address":    address,
			"latency_ms": latency,
		},
	}
}
