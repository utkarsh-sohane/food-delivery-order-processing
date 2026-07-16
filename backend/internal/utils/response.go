package utils

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// successResponse is the envelope for all successful responses.
type successResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Meta    interface{} `json:"meta,omitempty"`
}

// errorResponse is the envelope for all error responses.
type errorResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// paginationMeta holds pagination metadata returned alongside list responses.
type PaginationMeta struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"totalPages"`
}

// RespondSuccess writes a 200 OK response with the given data.
func RespondSuccess(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, successResponse{Success: true, Data: data})
}

// RespondCreated writes a 201 Created response with the given data.
func RespondCreated(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, successResponse{Success: true, Data: data})
}

// RespondList writes a 200 OK response with data and pagination metadata.
func RespondList(c *gin.Context, data interface{}, meta PaginationMeta) {
	c.JSON(http.StatusOK, successResponse{Success: true, Data: data, Meta: meta})
}

// RespondError writes an error response with the given HTTP status code and message.
func RespondError(c *gin.Context, statusCode int, message string) {
	c.JSON(statusCode, errorResponse{Success: false, Message: message})
}

// RespondBadRequest is a convenience wrapper for 400.
func RespondBadRequest(c *gin.Context, message string) {
	RespondError(c, http.StatusBadRequest, message)
}

// RespondNotFound is a convenience wrapper for 404.
func RespondNotFound(c *gin.Context, message string) {
	RespondError(c, http.StatusNotFound, message)
}

// RespondInternalError is a convenience wrapper for 500.
func RespondInternalError(c *gin.Context, message string) {
	RespondError(c, http.StatusInternalServerError, message)
}

// ComputeTotalPages returns the ceiling of total / limit.
func ComputeTotalPages(total int64, limit int) int {
	if limit <= 0 {
		return 0
	}
	pages := int(total) / limit
	if int(total)%limit != 0 {
		pages++
	}
	return pages
}
