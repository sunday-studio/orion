package utils

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// APIResponse represents a standard API response structure
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// PaginationMeta describes a paginated list in a UI-friendly, consistent shape.
type PaginationMeta struct {
	TotalItems     int64 `json:"total_items"`
	CurrentCount   int   `json:"current_count"`
	Limit          int   `json:"limit"`
	Offset         int   `json:"offset"`
	CurrentPage    int   `json:"current_page"`
	TotalPages     int   `json:"total_pages"`
	RangeStart     int   `json:"range_start"`
	RangeEnd       int   `json:"range_end"`
	HasNext        bool  `json:"has_next"`
	HasPrevious    bool  `json:"has_previous"`
	NextOffset     *int  `json:"next_offset,omitempty"`
	PreviousOffset *int  `json:"previous_offset,omitempty"`
}

// NewPaginationMeta creates pagination metadata for offset-based list endpoints.
func NewPaginationMeta(totalItems int64, limit int, offset int, currentCount int) PaginationMeta {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	if currentCount < 0 {
		currentCount = 0
	}

	currentPage := (offset / limit) + 1
	totalPages := 0
	if totalItems > 0 {
		totalPages = int((totalItems + int64(limit) - 1) / int64(limit))
	}

	rangeStart := 0
	rangeEnd := 0
	if currentCount > 0 {
		rangeStart = offset + 1
		rangeEnd = offset + currentCount
		if int64(rangeEnd) > totalItems {
			rangeEnd = int(totalItems)
		}
	}

	hasPrevious := offset > 0
	hasNext := int64(offset+currentCount) < totalItems
	var previousOffset *int
	if hasPrevious {
		value := offset - limit
		if value < 0 {
			value = 0
		}
		previousOffset = &value
	}
	var nextOffset *int
	if hasNext {
		value := offset + limit
		nextOffset = &value
	}

	return PaginationMeta{
		TotalItems:     totalItems,
		CurrentCount:   currentCount,
		Limit:          limit,
		Offset:         offset,
		CurrentPage:    currentPage,
		TotalPages:     totalPages,
		RangeStart:     rangeStart,
		RangeEnd:       rangeEnd,
		HasNext:        hasNext,
		HasPrevious:    hasPrevious,
		NextOffset:     nextOffset,
		PreviousOffset: previousOffset,
	}
}

// SuccessResponse sends a successful JSON response
func SuccessResponse(c *gin.Context, statusCode int, message string, data interface{}) {
	c.JSON(statusCode, APIResponse{
		Success: true,
		Message: message,
		Data:    data,
	})
}

// ErrorResponse sends an error JSON response
func ErrorResponse(c *gin.Context, statusCode int, message string, err error) {
	errorMsg := ""
	if err != nil {
		errorMsg = err.Error()
	}

	c.JSON(statusCode, APIResponse{
		Success: false,
		Message: message,
		Error:   errorMsg,
	})
}

// BadRequest sends a 400 Bad Request response
func BadRequest(c *gin.Context, message string) {
	ErrorResponse(c, http.StatusBadRequest, message, nil)
}

// Unauthorized sends a 401 Unauthorized response
func Unauthorized(c *gin.Context, message string) {
	ErrorResponse(c, http.StatusUnauthorized, message, nil)
}

// NotFound sends a 404 Not Found response
func NotFound(c *gin.Context, message string) {
	ErrorResponse(c, http.StatusNotFound, message, nil)
}

// InternalError sends a 500 Internal Server Error response
func InternalError(c *gin.Context, message string, err error) {
	ErrorResponse(c, http.StatusInternalServerError, message, err)
}
