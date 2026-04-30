package handler

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// Responses handles OpenAI Responses API endpoint for Anthropic platform groups.
// POST /v1/responses
// This converts Responses API requests to Anthropic format, forwards to Anthropic
// upstream, and converts responses back to Responses format.
func (h *GatewayHandler) Responses(c *gin.Context) {
	h.handleOpenAICompatEndpoint(c, openAICompatEndpointSpec{
		logName:              "handler.gateway.responses",
		logPrefix:            "gateway.responses",
		parseKind:            "responses",
		errorResponse:        h.responsesErrorResponse,
		failoverExhausted:    h.handleResponsesFailoverExhausted,
		forward:              h.gatewayService.ForwardAsResponses,
		restrictionErrorText: "This group is restricted to Claude Code clients (/v1/messages only)",
	})
}

// responsesErrorResponse writes an error in OpenAI Responses API format.
func (h *GatewayHandler) responsesErrorResponse(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{
		"error": gin.H{
			"code":    code,
			"message": message,
		},
	})
}

// handleResponsesFailoverExhausted writes a failover-exhausted error in Responses format.
func (h *GatewayHandler) handleResponsesFailoverExhausted(c *gin.Context, lastErr *service.UpstreamFailoverError, streamStarted bool) {
	if streamStarted {
		return // Can't write error after stream started
	}
	statusCode := http.StatusBadGateway
	if lastErr != nil && lastErr.StatusCode > 0 {
		statusCode = lastErr.StatusCode
	}
	h.responsesErrorResponse(c, statusCode, "server_error", "All available accounts exhausted")
}
