package admin

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type DataManagementHandler struct {
	dataManagementService dataManagementHealthService
}

func NewDataManagementHandler(dataManagementService *service.DataManagementService) *DataManagementHandler {
	return &DataManagementHandler{dataManagementService: dataManagementService}
}

type dataManagementHealthService interface {
	GetAgentHealth(ctx context.Context) service.DataManagementAgentHealth
}

func (h *DataManagementHandler) GetAgentHealth(c *gin.Context) {
	health := service.DataManagementAgentHealth{
		Enabled:    false,
		Reason:     service.DataManagementDeprecatedReason,
		SocketPath: service.DefaultDataManagementAgentSocketPath,
	}
	if h != nil && h.dataManagementService != nil {
		health = h.dataManagementService.GetAgentHealth(c.Request.Context())
	}

	payload := gin.H{
		"enabled":     health.Enabled,
		"reason":      health.Reason,
		"socket_path": health.SocketPath,
	}
	if health.Agent != nil {
		payload["agent"] = gin.H{
			"status":         health.Agent.Status,
			"version":        health.Agent.Version,
			"uptime_seconds": health.Agent.UptimeSeconds,
		}
	}
	response.Success(c, payload)
}
