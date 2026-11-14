//go:build unit

package service

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGrokAPIKeyAuthFailuresStayInvalidAcrossHealthPaths(t *testing.T) {
	for _, baseURL := range []string{"https://api.x.ai/v1", "https://grok-proxy.example/v1"} {
		for _, tc := range []struct {
			name   string
			status int
			body   string
		}{
			{"unauthorized", http.StatusUnauthorized, `{"error":"Unauthorized"}`},
			{"incorrect key 400", http.StatusBadRequest, `{"code":"invalid-argument","error":"Incorrect API key provided."}`},
			{"invalid key 403", http.StatusForbidden, `{"error":{"message":"Invalid API key provided."}}`},
		} {
			t.Run(baseURL+"/"+tc.name, func(t *testing.T) {
				account := &Account{
					ID: 2181, Platform: PlatformGrok, Type: AccountTypeAPIKey, Schedulable: true,
					Credentials: map[string]any{"api_key": "xai-invalid", "base_url": baseURL},
				}
				body := []byte(tc.body)
				require.Equal(t, APIKeyStatusActionPermanentDisable, ClassifyAPIKeyStatusAction(account, tc.status, body))

				probe := &AccountTestService{httpUpstream: &queuedHTTPUpstream{
					responses: []*http.Response{newJSONResponse(tc.status, tc.body)},
				}}
				health, err := probe.CheckAPIKeyValidity(context.Background(), account)
				require.NoError(t, err)
				require.False(t, health.Valid)
				require.True(t, health.Invalid)
				require.Equal(t, tc.status, health.StatusCode)

				manualRepo := &rateLimitAccountRepoStubWithSchedulable{}
				applyTestConnectionAction(context.Background(), manualRepo, account, tc.status, nil, body, "grok-4.7")
				require.Equal(t, 1, manualRepo.setErrorCalls)
				require.Equal(t, 1, manualRepo.setSchedulableCalls)
				require.False(t, manualRepo.lastSchedulable)
				require.Zero(t, manualRepo.tempCalls)
				require.Empty(t, manualRepo.modelRateLimitScopes)

				runtimeRepo := &rateLimitAccountRepoStubWithSchedulable{}
				runtime := &RateLimitService{accountRepo: runtimeRepo}
				require.True(t, runtime.HandleUpstreamError(context.Background(), account, tc.status, nil, body, "grok-4.7"))
				require.Equal(t, 1, runtimeRepo.setErrorCalls)
				require.Equal(t, 1, runtimeRepo.setSchedulableCalls)
				require.False(t, runtimeRepo.lastSchedulable)
				require.Zero(t, runtimeRepo.tempCalls)
				require.Empty(t, runtimeRepo.modelRateLimitScopes)
			})
		}
	}
}
