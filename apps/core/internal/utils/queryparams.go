package utils

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func QueryInt(c *gin.Context, key string, fallback int) int {
	value, err := strconv.Atoi(c.DefaultQuery(key, strconv.Itoa(fallback)))
	if err != nil || value < 0 {
		return fallback
	}
	return value
}

func QueryBool(c *gin.Context, key string, fallback bool) bool {
	value := strings.TrimSpace(c.Query(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func QueryOptionalBool(c *gin.Context, key string) *bool {
	value := strings.TrimSpace(c.Query(key))
	if value == "" {
		return nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return nil
	}
	return &parsed
}

func QueryStatuses(value string) []string {
	statuses := []string{}
	for _, status := range strings.Split(value, ",") {
		status = strings.TrimSpace(status)
		if status != "" {
			statuses = append(statuses, status)
		}
	}
	if len(statuses) == 0 {
		return []string{"open", "acknowledged", "covered"}
	}
	return statuses
}

func NonNegativeDurationSeconds(duration time.Duration) int64 {
	if duration < 0 {
		return 0
	}
	return int64(duration.Seconds())
}
