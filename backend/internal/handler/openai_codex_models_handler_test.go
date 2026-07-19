package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type emptyCodexModelsSchedulerCache struct {
	service.SchedulerCache
}

func (emptyCodexModelsSchedulerCache) GetSnapshot(context.Context, service.SchedulerBucket) ([]*service.Account, bool, error) {
	return nil, true, nil
}

func TestCodexModelsDoesNotReturnOpenAIListAsManifest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	schedulerSnapshot := service.NewSchedulerSnapshotService(
		emptyCodexModelsSchedulerCache{},
		nil,
		nil,
		nil,
		nil,
	)
	gatewayService := service.NewOpenAIGatewayService(
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		schedulerSnapshot,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	handler := &OpenAIGatewayHandler{gatewayService: gatewayService}

	groupID := int64(11)
	apiKey := &service.APIKey{
		GroupID: &groupID,
		Group: &service.Group{
			ID:       groupID,
			Platform: service.PlatformOpenAI,
		},
	}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyAPIKey), apiKey)
		c.Next()
	})
	router.GET("/v1/models", handler.CodexModels)
	router.GET("/backend-api/codex/models", handler.CodexModels)

	for _, path := range []string{
		"/v1/models?client_version=0.145.0-alpha.18",
		"/backend-api/codex/models?client_version=0.145.0-alpha.18",
	} {
		t.Run(path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, path, nil)

			router.ServeHTTP(recorder, request)

			require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
			require.Contains(t, recorder.Body.String(), "No available OpenAI OAuth accounts")
			require.NotContains(t, recorder.Body.String(), `"object":"list"`)
		})
	}
}
