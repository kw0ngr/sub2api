//go:build unit

package service

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestRateLimitService_HandleUpstreamError_DeepSeekModel404UsesModelCooldown(t *testing.T) {
	repo := &rateLimitAccountRepoStub{}
	svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	account := &Account{
		ID:          808,
		Platform:    PlatformAnthropic,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Credentials: map[string]any{"api_key": "sk-test"},
	}

	handled := svc.HandleUpstreamError(
		context.Background(),
		account,
		http.StatusNotFound,
		http.Header{},
		[]byte(`{"error":{"message":"model: deepseek-v4-pro","type":"server_error"}}`),
		"deepseek-v4-pro",
	)

	require.True(t, handled)
	require.Equal(t, []string{"deepseek-v4-pro"}, repo.modelRateLimitScopes)
	require.Len(t, repo.modelRateLimitResets, 1)
	require.WithinDuration(t, time.Now().Add(30*time.Minute), repo.modelRateLimitResets[0], 5*time.Second)
	require.Zero(t, repo.tempCalls)
	require.Zero(t, repo.rateLimitedCalls)
	require.Zero(t, repo.setErrorCalls)
}
