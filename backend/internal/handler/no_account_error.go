package handler

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type noAccountErrorClassification struct {
	Status        int
	ErrType       string
	Message       string
	ModelNotFound bool
}

type noAccountDiagnosisRequest struct {
	Diagnoser    service.ModelAvailabilityDiagnoser
	APIKey       *service.APIKey
	RoutingModel string
	DisplayModel string
	Platform     string
}

func classifyNoAccountError(ctx context.Context, req noAccountDiagnosisRequest) noAccountErrorClassification {
	fallback := noAccountErrorClassification{
		Status:  http.StatusServiceUnavailable,
		ErrType: "api_error",
		Message: "Service temporarily unavailable",
	}

	routingModel := strings.TrimSpace(req.RoutingModel)
	displayModel := strings.TrimSpace(req.DisplayModel)
	if displayModel == "" {
		displayModel = routingModel
	}
	if req.Diagnoser == nil || req.APIKey == nil || req.APIKey.GroupID == nil || routingModel == "" {
		return fallback
	}

	result := req.Diagnoser.DiagnoseModelAvailabilityForPlatform(ctx, req.APIKey.GroupID, routingModel, req.Platform)
	if result.HasAccountsInPool && !result.HasModelSupport {
		return noAccountErrorClassification{
			Status:        http.StatusNotFound,
			ErrType:       "model_not_found",
			Message:       fmt.Sprintf("Model %q is not supported by any configured account in this group", displayModel),
			ModelNotFound: true,
		}
	}
	return fallback
}

func classifyNoAccountErrorFromGin(c *gin.Context, req noAccountDiagnosisRequest) noAccountErrorClassification {
	ctx := context.Background()
	if c != nil && c.Request != nil {
		ctx = c.Request.Context()
	}
	return classifyNoAccountError(ctx, req)
}
