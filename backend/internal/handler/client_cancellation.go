package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// Only the inbound request's context proves client cancellation, not an upstream timeout.
func abortCanceledGatewayRequest(c *gin.Context) bool {
	if c == nil || c.Request == nil || c.Request.Context().Err() == nil {
		return false
	}
	c.Set(service.OpsClientCanceledKey, true)
	if !c.Writer.Written() {
		c.Status(499)
	}
	c.Abort()
	return true
}
