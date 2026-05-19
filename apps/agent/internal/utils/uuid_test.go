package utils

import (
	"errors"
	"net"
	"os"
	"runtime"
	"testing"
)

func TestGenerateAgentUUIDFallsBackToMachineID(t *testing.T) {
	originalHostname := hostnameFunc
	originalInterfaces := networkInterfacesFunc
	originalReadFile := readFileFunc
	t.Cleanup(func() {
		hostnameFunc = originalHostname
		networkInterfacesFunc = originalInterfaces
		readFileFunc = originalReadFile
	})

	hostnameFunc = func() (string, error) {
		return "nowhere", nil
	}
	networkInterfacesFunc = func() ([]net.Interface, error) {
		return nil, errors.New("route ip+net: netlinkrib: address family not supported by protocol")
	}
	readFileFunc = func(path string) ([]byte, error) {
		if path == "/etc/machine-id" {
			return []byte("machine-123\n"), nil
		}
		return nil, os.ErrNotExist
	}

	got, err := GenerateAgentUUID()
	if err != nil {
		t.Fatalf("GenerateAgentUUID() error = %v", err)
	}
	if got == "" {
		t.Fatal("GenerateAgentUUID() returned empty UUID")
	}
}

func TestMachineIdentityFallsBackToRuntime(t *testing.T) {
	originalInterfaces := networkInterfacesFunc
	originalReadFile := readFileFunc
	t.Cleanup(func() {
		networkInterfacesFunc = originalInterfaces
		readFileFunc = originalReadFile
	})

	networkInterfacesFunc = func() ([]net.Interface, error) {
		return nil, errors.New("interfaces unavailable")
	}
	readFileFunc = func(path string) ([]byte, error) {
		return nil, os.ErrNotExist
	}

	want := "host:" + runtime.GOOS + ":" + runtime.GOARCH
	if got := machineIdentity(); got != want {
		t.Fatalf("machineIdentity() = %q, want %q", got, want)
	}
}
