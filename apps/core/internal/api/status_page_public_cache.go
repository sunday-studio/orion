package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"orion/core/internal/utils"

	"github.com/gin-gonic/gin"
)

const statusPagePublicCacheControl = "public, max-age=30, stale-while-revalidate=30"

func (s *Server) writePublicStatusPageJSON(c *gin.Context, statusCode int, message string, data interface{}) {
	payload, err := json.Marshal(utils.APIResponse{
		Success: true,
		Message: message,
		Data:    data,
	})
	if err != nil {
		s.logger.Error("Failed to render public status page response", "error", err)
		utils.InternalError(c, "Failed to render status page response", err)
		return
	}
	writePublicStatusPagePayload(c, statusCode, "application/json; charset=utf-8", payload)
}

func writePublicStatusPagePayload(c *gin.Context, statusCode int, contentType string, payload []byte) {
	etag := publicStatusPageETag(payload)
	c.Header("Cache-Control", statusPagePublicCacheControl)
	c.Header("ETag", etag)

	if statusCode == http.StatusOK && publicStatusPageETagMatches(c.GetHeader("If-None-Match"), etag) {
		c.Status(http.StatusNotModified)
		return
	}

	c.Data(statusCode, contentType, payload)
}

func publicStatusPageETag(payload []byte) string {
	sum := sha256.Sum256(payload)
	return `"` + hex.EncodeToString(sum[:]) + `"`
}

func publicStatusPageETagMatches(header string, etag string) bool {
	for _, candidate := range strings.Split(header, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" || candidate == etag || strings.TrimPrefix(candidate, "W/") == etag {
			return true
		}
	}
	return false
}
