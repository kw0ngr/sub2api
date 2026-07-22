package handler

import (
	"context"
	"encoding/json"
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

type localCodexModelsHandlerRepo struct {
	service.AccountRepository
	accounts []service.Account
}

func (r localCodexModelsHandlerRepo) ListSchedulableByGroupIDAndPlatform(_ context.Context, _ int64, platform string) ([]service.Account, error) {
	return r.filterPlatform(platform), nil
}

func (r localCodexModelsHandlerRepo) ListSchedulableByPlatform(_ context.Context, platform string) ([]service.Account, error) {
	return r.filterPlatform(platform), nil
}

func (r localCodexModelsHandlerRepo) filterPlatform(platform string) []service.Account {
	result := make([]service.Account, 0, len(r.accounts))
	for _, account := range r.accounts {
		if account.Platform == platform {
			result = append(result, account)
		}
	}
	return result
}

func newCodexModelsHandlerGatewayService(repo service.AccountRepository, schedulerSnapshot *service.SchedulerSnapshotService) *service.OpenAIGatewayService {
	return service.NewOpenAIGatewayService(
		repo,
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
	gatewayService := newCodexModelsHandlerGatewayService(nil, schedulerSnapshot)
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
			require.Contains(t, recorder.Body.String(), "No available OpenAI accounts")
			require.NotContains(t, recorder.Body.String(), `"object":"list"`)
		})
	}
}

func TestCodexModelsServesGroupScopedManifestForAPIKeyPool(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := localCodexModelsHandlerRepo{accounts: []service.Account{
		{
			ID:          581,
			Platform:    service.PlatformOpenAI,
			Type:        service.AccountTypeAPIKey,
			Status:      service.StatusActive,
			Schedulable: true,
			Credentials: map[string]any{
				"model_mapping": map[string]any{
					"gpt-5.4":     "gpt-5.4",
					"gpt-5.6-sol": "gpt-5.6-sol",
				},
			},
		},
	}}
	handler := &OpenAIGatewayHandler{gatewayService: newCodexModelsHandlerGatewayService(repo, nil)}

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

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/models?client_version=0.145.0", nil)
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.NotEmpty(t, recorder.Header().Get("ETag"))
	require.NotContains(t, recorder.Body.String(), `"object":"list"`)

	var envelope struct {
		Models []struct {
			Slug                     string `json:"slug"`
			DefaultReasoningLevel    string `json:"default_reasoning_level"`
			SupportedReasoningLevels []struct {
				Effort string `json:"effort"`
			} `json:"supported_reasoning_levels"`
		} `json:"models"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Len(t, envelope.Models, 2)
	require.Equal(t, "gpt-5.6-sol", envelope.Models[0].Slug)
	require.Equal(t, "low", envelope.Models[0].DefaultReasoningLevel)
	require.Equal(t, "max", envelope.Models[0].SupportedReasoningLevels[len(envelope.Models[0].SupportedReasoningLevels)-1].Effort)

	notModifiedRecorder := httptest.NewRecorder()
	notModifiedRequest := httptest.NewRequest(http.MethodGet, "/v1/models?client_version=0.145.0", nil)
	notModifiedRequest.Header.Set("If-None-Match", recorder.Header().Get("ETag"))
	router.ServeHTTP(notModifiedRecorder, notModifiedRequest)
	require.Equal(t, http.StatusNotModified, notModifiedRecorder.Code)
}
