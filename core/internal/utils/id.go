package utils

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

func IsValidID(value string, prefix string) bool {
	parts := strings.SplitN(value, "-", 2)
	if len(parts) != 2 {
		return false
	}
	idPrefix := parts[0]
	idUUID := parts[1]

	if idPrefix != prefix {
		return false
	}
	_, err := uuid.Parse(idUUID)
	return err == nil
}

func GenerateID(prefix string) string {
	return fmt.Sprintf("%s-%s", prefix, uuid.New().String())
}
