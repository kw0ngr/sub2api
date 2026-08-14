//go:build unit

package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

var glmNetworkErrorBody = []byte(`{"type":"error","error":{"code":"1234","type":"overloaded_error","message":"[1234][网络错误，请稍后重试。]"}}`)

func TestRateLimitService_HandleUpstreamError_GLMNetworkErrorUsesModelCooldown(t *testing.T) {
	// Given
	repo := &rateLimitAccountRepoStub{}
	svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	account := &Account{ID: 1784, Platform: PlatformGLM, Type: AccountTypeAPIKey}

	// When
	before := time.Now()
	handled := svc.HandleUpstreamError(context.Background(), account, http.StatusForbidden, nil, glmNetworkErrorBody, "glm-5.3")
	after := time.Now()

	// Then
	require.True(t, handled)
	require.Zero(t, repo.tempCalls, "a model-scoped network error must not cool the whole GLM account")
	require.Equal(t, []string{"glm-5.3"}, repo.modelRateLimitScopes)
	require.Len(t, repo.modelRateLimitResets, 1)
	require.WithinDuration(t, before.Add(30*time.Second), repo.modelRateLimitResets[0], after.Sub(before)+time.Second)
}

func TestGatewayService_streamSSEErrorFailover_passesGLMModelToHealthPolicy(t *testing.T) {
	// Given
	gin.SetMode(gin.TestMode)
	repo := &rateLimitAccountRepoStub{}
	svc := newMinimalGatewayService()
	svc.rateLimitService = NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	account := &Account{ID: 1784, Platform: PlatformGLM, Type: AccountTypeAPIKey}
	resp := &http.Response{Header: http.Header{}}

	// When
	svc.streamSSEErrorFailover(context.Background(), c, account, resp, &streamSSEError{body: glmNetworkErrorBody}, "glm-5.3")

	// Then
	require.Zero(t, repo.tempCalls)
	require.Equal(t, []string{"glm-5.3"}, repo.modelRateLimitScopes)
}
