package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCodexModelsRoutesEmitAstraAndSolExactlyOnceForAPIKeyGroup(t *testing.T) {
	// Given
	gin.SetMode(gin.TestMode)
	groupID := int64(901)
	repo := localCodexModelsHandlerRepo{accounts: []service.Account{{
		ID: 801, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true,
		Credentials: map[string]any{"model_mapping": map[string]any{
			"gpt-6-astra": "gpt-6-astra",
			"gpt-5.6":     "gpt-5.6-sol",
			"gpt-5.6-sol": "gpt-5.6-sol",
		}},
	}}}
	h := &OpenAIGatewayHandler{gatewayService: newCodexModelsHandlerGatewayService(repo, nil)}
	apiKey := &service.APIKey{GroupID: &groupID, Group: &service.Group{ID: groupID, Platform: service.PlatformOpenAI}}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyAPIKey), apiKey)
		c.Next()
	})
	router.GET("/v1/models", h.CodexModels)
	router.GET("/backend-api/codex/models", h.CodexModels)

	for _, path := range []string{"/v1/models?client_version=0.153.4", "/backend-api/codex/models?client_version=0.153.4"} {
		t.Run(path, func(t *testing.T) {
			// When
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))

			// Then
			require.Equal(t, http.StatusOK, recorder.Code)
			counts := task5HandlerSlugCounts(t, recorder.Body.Bytes())
			require.Equal(t, 1, counts["gpt-6-astra"])
			require.Equal(t, 1, counts["gpt-5.6-sol"])
			require.Zero(t, counts["gpt-5.6"])
			require.Zero(t, counts["gpt5.6"])
			require.Contains(t, recorder.Body.String(), `"use_responses_lite":false`)
		})
	}
}

func task5HandlerSlugCounts(t *testing.T, body []byte) map[string]int {
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
