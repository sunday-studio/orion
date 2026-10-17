package utils

import (
	"crypto/md5"
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"
)

var (
	hostnameFunc          = os.Hostname
	networkInterfacesFunc = net.Interfaces
	readFileFunc          = os.ReadFile
)

// GenerateAgentUUID creates a unique UUID for the agent based on hostname and MAC address
// This ensures the same machine always gets the same UUID across restarts
func GenerateAgentUUID() (string, error) {
	// Get hostname
	hostname, err := hostnameFunc()
	if err != nil {
		return "", fmt.Errorf("failed to get hostname: %w", err)
	}

	combined := fmt.Sprintf("%s-%s", hostname, machineIdentity())

	hash := md5.Sum([]byte(combined))
	uuid := fmt.Sprintf("%x-%x-%x-%x-%x",
		hash[0:4], hash[4:6], hash[6:8], hash[8:10], hash[10:16])

	return uuid, nil
}

func machineIdentity() string {
	if macAddr := firstMACAddress(); macAddr != "" {
		return "mac:" + macAddr
	}
	if machineID := linuxMachineID(); machineID != "" {
		return "machine-id:" + machineID
	}
	return fmt.Sprintf("host:%s:%s", runtime.GOOS, runtime.GOARCH)
}

func firstMACAddress() string {
	interfaces, err := networkInterfacesFunc()
	if err != nil {
		return ""
	}

	for _, iface := range interfaces {
		if len(iface.HardwareAddr) > 0 {
			return iface.HardwareAddr.String()
		}
	}
	return ""
}

func linuxMachineID() string {
	for _, path := range []string{"/etc/machine-id", "/var/lib/dbus/machine-id"} {
		data, err := readFileFunc(path)
		if err != nil {
			continue
		}
		if value := strings.TrimSpace(string(data)); value != "" {
			return value
		}
	}
	return ""
}

func GetSystemInfo() (name, osName, arch string, err error) {
	hostname, err := os.Hostname()
	if err != nil {
		return "", "", "", fmt.Errorf("failed to get hostname: %w", err)
	}

	return hostname, runtime.GOOS, runtime.GOARCH, nil
}
