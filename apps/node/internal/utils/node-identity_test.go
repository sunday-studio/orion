package utils

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

// stubMachine fakes hostname, MAC and /etc/machine-id; other files read normally.
func stubMachine(t *testing.T, hostname string, mac string, machineID string) {
	t.Helper()
	oh, oi, or := hostnameFunc, networkInterfacesFunc, readFileFunc
	t.Cleanup(func() { hostnameFunc, networkInterfacesFunc, readFileFunc = oh, oi, or })
	hostnameFunc = func() (string, error) { return hostname, nil }
	networkInterfacesFunc = func() ([]net.Interface, error) {
		if mac == "" {
			return nil, nil
		}
		hw, _ := net.ParseMAC(mac)
		return []net.Interface{{HardwareAddr: hw}}, nil
	}
	readFileFunc = func(path string) ([]byte, error) {
		if path == "/etc/machine-id" || path == "/var/lib/dbus/machine-id" {
			if machineID == "" {
				return nil, os.ErrNotExist
			}
			return []byte(machineID + "\n"), nil
		}
		return os.ReadFile(path)
	}
}

func TestResolveNodeIdentityIsStableAndKeepsLegacyIDForContinuity(t *testing.T) {
	stubMachine(t, "web-1", "02:42:ac:11:00:02", "mid-aaa")
	path := filepath.Join(t.TempDir(), "machine-identity.json")
	legacy, _ := GenerateNodeUUID()

	first, err := ResolveNodeIdentity(path)
	if err != nil || first != legacy {
		t.Fatalf("first id = %q err = %v, want legacy id %q so reinstalls reconnect", first, err, legacy)
	}
	second, _ := ResolveNodeIdentity(path)
	if second != first {
		t.Fatalf("id changed across calls: %q then %q", first, second)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("identity file mode = %v err = %v, want 0600", info.Mode().Perm(), err)
	}
}

// S9: a cloned VM (same MAC, copied state directory) gets a distinct identity.
func TestResolveNodeIdentityGivesClonedMachineDistinctID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "machine-identity.json")
	stubMachine(t, "web-1", "02:42:ac:11:00:02", "mid-aaa")
	original, _ := ResolveNodeIdentity(path)

	for name, clone := range map[string][2]string{
		"different hostname":   {"web-2", "mid-aaa"},
		"different machine-id": {"web-1", "mid-bbb"},
	} {
		t.Run(name, func(t *testing.T) {
			stubMachine(t, clone[0], "02:42:ac:11:00:02", clone[1]) // identical MAC
			cloned, err := ResolveNodeIdentity(path)
			if err != nil || cloned == original {
				t.Fatalf("clone id = %q (err %v), want it to differ from %q", cloned, err, original)
			}
			again, _ := ResolveNodeIdentity(path)
			if again != cloned {
				t.Fatalf("clone id not stable after reissue: %q vs %q", again, cloned)
			}
			// reset the file for the next subtest
			stubMachine(t, "web-1", "02:42:ac:11:00:02", "mid-aaa")
			os.WriteFile(path, []byte(`{"id":"`+original+`","hostname":"web-1","machine_id":"mid-aaa"}`), 0o600)
		})
	}
}

func TestResolveNodeIdentityIsRandomWithoutAnyMachineIdentity(t *testing.T) {
	stubMachine(t, "container", "", "")
	a, _ := ResolveNodeIdentity(filepath.Join(t.TempDir(), "id.json"))
	b, _ := ResolveNodeIdentity(filepath.Join(t.TempDir(), "id.json"))
	if a == "" || a == b {
		t.Fatalf("bare containers must not collide: %q vs %q", a, b)
	}
}

func TestResolveNodeIdentityDistrustsCorruptFileAndSurvivesUnwritableDir(t *testing.T) {
	stubMachine(t, "web-1", "02:42:ac:11:00:02", "mid-aaa")
	dir := t.TempDir()
	path := filepath.Join(dir, "id.json")
	os.WriteFile(path, []byte("not json"), 0o600)
	legacy, _ := GenerateNodeUUID()
	got, err := ResolveNodeIdentity(path)
	if err != nil || got == "" || got == legacy {
		t.Fatalf("corrupt file id = %q err = %v, want a fresh random id", got, err)
	}
	if id, err := ResolveNodeIdentity("/proc/definitely/not/writable/id.json"); err != nil || id == "" {
		t.Fatalf("unwritable path id = %q err = %v, want registration to proceed", id, err)
	}
}
