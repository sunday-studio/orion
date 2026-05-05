package utils

import (
	"crypto/rand"
	"encoding/hex"
)

// GenerateToken creates a secure random token for agent authentication
func GenerateToken() (string, error) {
	// Generate 32 random bytes (256 bits)
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	// Convert to hex string
	return hex.EncodeToString(bytes), nil
}
