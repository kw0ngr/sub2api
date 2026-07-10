package handler

import (
	"errors"
	"net/http"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func shouldFallbackCodexModelsToV1List(fullPath string, err error) bool {
	return fullPath == "/v1/models" && errors.Is(err, service.ErrNoAvailableAccounts)
}

func (h *OpenAIGatewayHandler) CodexModels(c *gin.Context) {
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok || apiKey.Group == nil {
		h.errorResponse(c, http.StatusUnauthorized, "invalid_request_error", "API key group is required")
		return
	}
	if apiKey.Group.Platform != service.PlatformOpenAI {
		h.errorResponse(c, http.StatusNotFound, "not_found_error", "Codex models manifest is only available for OpenAI groups")
		return
	}

	account, err := h.gatewayService.SelectCodexModelsAccount(c.Request.Context(), apiKey.GroupID)
	if err != nil {
		if shouldFallbackCodexModelsToV1List(c.FullPath(), err) {
			c.JSON(http.StatusOK, gin.H{
				"object": "list",
				"data":   openai.DefaultModels,
			})
			return
		}
		h.errorResponse(c, http.StatusServiceUnavailable, "upstream_error", "No available OpenAI OAuth accounts")
		return
	}

	manifest, err := h.gatewayService.FetchCodexModelsManifest(c.Request.Context(), account, c.Query("client_version"), c.GetHeader("If-None-Match"))
	if err != nil {
		h.errorResponse(c, infraerrors.Code(err), "upstream_error", infraerrors.Message(err))
		return
	}
	if manifest.ETag != "" {
		c.Header("ETag", manifest.ETag)
	}
	if manifest.NotModified {
		c.Status(http.StatusNotModified)
		return
	}
	c.Data(http.StatusOK, "application/json", manifest.Body)
}
