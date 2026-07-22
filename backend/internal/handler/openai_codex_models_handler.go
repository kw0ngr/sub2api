package handler

import (
	"errors"
	"net/http"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *OpenAIGatewayHandler) CodexModels(c *gin.Context) {
	if c.Request.Context().Err() != nil {
		return
	}
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok || apiKey.Group == nil {
		h.errorResponse(c, http.StatusUnauthorized, "invalid_request_error", "API key group is required")
		return
	}
	if apiKey.Group.Platform != service.PlatformOpenAI {
		h.errorResponse(c, http.StatusNotFound, "not_found_error", "Codex models manifest is only available for OpenAI groups")
		return
	}

	// API-key pools cannot authenticate against ChatGPT's OAuth-only Codex
	// manifest endpoint. Serve a deterministic manifest from the group's own
	// schedulable model mappings before considering OAuth passthrough.
	localManifest, localErr := h.gatewayService.BuildLocalCodexModelsManifest(
		c.Request.Context(),
		apiKey.GroupID,
		c.GetHeader("If-None-Match"),
	)
	if localErr == nil {
		writeCodexModelsManifest(c, localManifest)
		return
	}
	if !errors.Is(localErr, service.ErrNoAvailableAccounts) {
		h.errorResponse(c, http.StatusInternalServerError, "api_error", "Failed to build Codex models manifest")
		return
	}

	maxAccountSwitches := h.maxAccountSwitches
	if maxAccountSwitches <= 0 {
		maxAccountSwitches = 3
	}
	excludedAccountIDs := make(map[int64]struct{})
	var lastUpstreamErr error

	for switchCount := 0; ; switchCount++ {
		account, err := h.gatewayService.SelectCodexModelsAccountWithExclusions(c.Request.Context(), apiKey.GroupID, excludedAccountIDs)
		if err != nil {
			if c.Request.Context().Err() != nil {
				return
			}
			if lastUpstreamErr != nil {
				h.errorResponse(c, infraerrors.Code(lastUpstreamErr), "upstream_error", infraerrors.Message(lastUpstreamErr))
				return
			}
			h.errorResponse(c, http.StatusServiceUnavailable, "upstream_error", "No available OpenAI accounts")
			return
		}
		setOpsSelectedAccount(c, account.ID, account.Platform)

		manifest, fetchErr := h.gatewayService.FetchCodexModelsManifest(c.Request.Context(), account, c.Query("client_version"), c.GetHeader("If-None-Match"))
		if fetchErr == nil {
			writeCodexModelsManifest(c, manifest)
			return
		}
		if c.Request.Context().Err() != nil {
			return
		}
		lastUpstreamErr = fetchErr
		if switchCount >= maxAccountSwitches || infraerrors.Code(fetchErr) < http.StatusInternalServerError {
			h.errorResponse(c, infraerrors.Code(fetchErr), "upstream_error", infraerrors.Message(fetchErr))
			return
		}
		excludedAccountIDs[account.ID] = struct{}{}
	}
}

func writeCodexModelsManifest(c *gin.Context, manifest *service.CodexModelsManifest) {
	if manifest == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"type": "api_error", "message": "Codex models manifest is empty"}})
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
