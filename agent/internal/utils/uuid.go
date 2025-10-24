package utils

import (
	"crypto/md5"
	"fmt"
	"net"
	"os"
	"runtime"
)

// GenerateAgentUUID creates a unique UUID for the agent based on hostname and MAC address
// This ensures the same machine always gets the same UUID across restarts
func GenerateAgentUUID() (string, error) {
	// Get hostname
	hostname, err := os.Hostname()
	if err != nil {
		return "", fmt.Errorf("failed to get hostname: %w", err)
	}

	// Get MAC address of the first network interface
	var macAddr string
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", fmt.Errorf("failed to get network interfaces: %w", err)
	}

	for _, iface := range interfaces {
		if iface.HardwareAddr != nil && len(iface.HardwareAddr) > 0 {
			macAddr = iface.HardwareAddr.String()
			break
		}
	}

	if macAddr == "" {
		return "", fmt.Errorf("no MAC address found")
	}

	// Combine hostname and MAC address to create a unique identifier
	combined := fmt.Sprintf("%s-%s", hostname, macAddr)
	
	// Generate MD5 hash and format as UUID-like string
	hash := md5.Sum([]byte(combined))
	uuid := fmt.Sprintf("%x-%x-%x-%x-%x", 
		hash[0:4], hash[4:6], hash[6:8], hash[8:10], hash[10:16])
	
	return uuid, nil
}

// GetSystemInfo returns basic system information for registration
func GetSystemInfo() (name, osName, arch string, err error) {
	hostname, err := os.Hostname()
	if err != nil {
		return "", "", "", fmt.Errorf("failed to get hostname: %w", err)
	}

	return hostname, runtime.GOOS, runtime.GOARCH, nil
}
