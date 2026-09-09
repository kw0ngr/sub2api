package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type task5RoutesCodexModelsRepo struct {
	service.AccountRepository
	accounts []service.Account
}

func (r task5RoutesCodexModelsRepo) ListSchedulableByGroupIDAndPlatform(_ context.Context, _ int64, platform string) ([]service.Account, error) {
	return task5RoutesFilterPlatform(r.accounts, platform), nil
}

func (r task5RoutesCodexModelsRepo) ListSchedulableByPlatform(_ context.Context, platform string) ([]service.Account, error) {
	return task5RoutesFilterPlatform(r.accounts, platform), nil
}

func task5RoutesFilterPlatform(accounts []service.Account, platform string) []service.Account {
	out := make([]service.Account, 0, len(accounts))
	for _, account := range accounts {
		if account.Platform == platform {
			out = append(out, account)
		}
	}
	return out
}

func TestModelsRouteCodexModelPathsEmitCanonicalAstraAndSol(t *testing.T) {
	// Given
	gin.SetMode(gin.TestMode)
	groupID := int64(921)
	repo := task5RoutesCodexModelsRepo{accounts: []service.Account{{
		ID: 811, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true,
		Credentials: map[string]any{"model_mapping": map[string]any{
			"gpt-6-astra": "gpt-6-astra",
			"gpt5.6":      "gpt-5.6-sol",
		}},
	}}}
	openAIService := service.NewOpenAIGatewayService(repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	router := gin.New()
	RegisterGatewayRoutes(
		router,
		&handler.Handlers{
			Gateway:       &handler.GatewayHandler{},
			OpenAIGateway: handler.NewOpenAIGatewayHandler(openAIService, nil, nil, nil, nil, nil, &config.Config{}),
		},
		servermiddleware.APIKeyAuthMiddleware(func(c *gin.Context) {
			c.Set(string(servermiddleware.ContextKeyAPIKey), &service.APIKey{GroupID: &groupID, Group: &service.Group{ID: groupID, Platform: service.PlatformOpenAI}})
			c.Next()
		}),
		nil,
		nil,
		nil,
		nil,
		&config.Config{},
	)

	for _, path := range []string{"/v1/models?client_version=0.153.4", "/backend-api/codex/models?client_version=0.153.4"} {
		t.Run(path, func(t *testing.T) {
			// When
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))

			// Then
			require.Equal(t, http.StatusOK, recorder.Code)
			counts := task5RouteSlugCounts(t, recorder.Body.Bytes())
			require.Equal(t, 1, counts["gpt-6-astra"])
			require.Equal(t, 1, counts["gpt-5.6-sol"])
			require.Zero(t, counts["gpt5.6"])
		})
	}
}

func task5RouteSlugCounts(t *testing.T, body []byte) map[string]int {
	t.Helper()
	var envelope struct {
		Models []struct {
			Slug string `json:"slug"`
		} `json:"models"`
	}
	require.NoError(t, json.Unmarshal(body, &envelope))
	counts := make(map[string]int, len(envelope.Models))
	for _, model := range envelope.Models {
		counts[model.Slug]++
	}
	return counts
}
