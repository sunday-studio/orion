package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// identityRecord is persisted beside the agent state. Hostname and machine-id are
// stored with the id so a copied state directory (a cloned VM or image) is
// recognised and given a fresh identity instead of impersonating the original.
type identityRecord struct {
	ID        string `json:"id"`
	Hostname  string `json:"hostname"`
	MachineID string `json:"machine_id"`
}

// ResolveAgentIdentity returns the stable machine id used to register with Core.
//
//   - A previously persisted id is reused while the hostname and /etc/machine-id
//     still match.
//   - If they differ, the state was cloned: a new random id is issued.
//   - On first run the legacy hostname+MAC derived id is kept when the machine has a
//     MAC or machine-id, so reinstalling an agent reconnects to the same Core agent
//     record; machines with neither (bare containers) get a random id.
func ResolveAgentIdentity(persistPath string) (string, error) {
	hostname, err := hostnameFunc()
	if err != nil {
		return "", fmt.Errorf("failed to get hostname: %w", err)
	}
	machineID := linuxMachineID()

	existing, hadFile := readIdentityRecord(persistPath)
	if hadFile && existing.ID != "" && existing.Hostname == hostname && existing.MachineID == machineID {
		return existing.ID, nil
	}

	var id string
	switch {
	case hadFile:
		id = newUUIDv4()
	case firstMACAddress() != "" || machineID != "":
		id, err = GenerateAgentUUID()
		if err != nil {
			return "", err
		}
	default:
		id = newUUIDv4()
	}

	// Persisting is best effort: an unwritable directory must not block registration.
	_ = writeIdentityRecord(persistPath, identityRecord{ID: id, Hostname: hostname, MachineID: machineID})
	return id, nil
}

func readIdentityRecord(path string) (identityRecord, bool) {
	data, err := readFileFunc(path)
	if err != nil {
		return identityRecord{}, false
	}
	var rec identityRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return identityRecord{}, true // unreadable file counts as tampered: do not trust it
	}
	return rec, true
}

func writeIdentityRecord(path string, rec identityRecord) error {
	data, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
