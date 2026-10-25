package utils

import (
	"fmt"

	"github.com/google/uuid"
)

func GenerateID(prefix string) string {
	return fmt.Sprintf("%s-%s", prefix, uuid.New().String())
}
