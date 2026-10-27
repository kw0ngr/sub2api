package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const cloudflare1010Body = `{"error":{"cloudflare_error":true,"code":1010,"message":"Request blocked"}}`

func TestIsCloudflareBotBlockResponse(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		want       bool
	}{
		{"html", "<html>Error code: 1010</html>", true},
		{"json_code", cloudflare1010Body, true},
		{"nested_array", `[{"error":{"Cloudflare_Error":true,"Error_Code":"1010"}}]`, true},
		{"name", `{"error":{"ERROR_NAME":"BROWSER_SIGNATURE_BANNED"}}`, true},
		{"unmarked_code", `{"error":{"code":1010,"message":"Account denied"}}`, false},
		{"false_marker", `{"cloudflare_error":false,"code":1010}`, false},
		{"different_code", `{"cloudflare_error":true,"code":1020}`, false},
		{"separate_objects", `{"cloudflare_error":true,"error":{"code":1010}}`, false},
		{"json_message_only", `{"message":"Error code: 1010"}`, false},
		{"ordinary_forbidden", `{"error":{"code":"permission_denied"}}`, false},
		{"empty", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, isCloudflareBotBlockResponse([]byte(tc.body)))
		})
	}
}

func TestCloudflare1010DoesNotMutateAccountHealth(t *testing.T) {
	for _, platform := range []string{PlatformOpenAI, PlatformAnthropic, PlatformGemini, PlatformAntigravity, PlatformGrok, PlatformGLM, PlatformDeepSeek} {
		for _, accountType := range []string{AccountTypeAPIKey, AccountTypeOAuth} {
			t.Run(platform+"/"+accountType, func(t *testing.T) {
				account := &Account{ID: 2165, Platform: platform, Type: accountType, Schedulable: true,
					Credentials: map[string]any{
						"custom_error_codes_enabled": true,
						"custom_error_codes":         []any{float64(403)},
						"temp_unschedulable_enabled": true,
						"temp_unschedulable_rules": []any{map[string]any{
							"error_code": float64(403), "keywords": []any{"cloudflare"}, "duration_minutes": float64(30),
						}},
					}}
				repo := &grokQuotaAccountRepoStub{account: account}
				svc := &RateLimitService{accountRepo: repo}
				ctx := context.Background()
				body := []byte(cloudflare1010Body)
				require.Equal(t, APIKeyStatusActionIgnore, ClassifyAPIKeyStatusAction(account, 403, body))
				require.False(t, svc.HandleUpstreamError(ctx, account, 403, nil, body, "test-model"))
				require.Equal(t, ErrorPolicyNone, svc.CheckErrorPolicy(ctx, account, 403, body))
				require.False(t, svc.HandleTempUnschedulable(ctx, account, 403, body))
				require.False(t, svc.handle403(ctx, account, "Request blocked", body))
				applyTestConnectionAction(ctx, repo, account, 403, nil, body, "test-model")
				if platform == PlatformGrok {
					(&OpenAIGatewayService{accountRepo: repo}).handleGrokAccountUpstreamError(ctx, account, 403, nil, body)
					(&GrokQuotaService{accountRepo: repo}).applyInferenceProbeSideEffects(ctx, account, nil, 403, nil, body)
				}
				require.True(t, account.Schedulable)
				require.Zero(t, repo.setErrorCalls)
				require.Zero(t, repo.setSchedulableN)
				require.Zero(t, repo.modelRateLimitN)
				require.Nil(t, repo.tempUnschedUntil)
				require.Nil(t, repo.rateLimitedUntil)
			})
		}
	}
}

func TestGrokModelsCloudflare1010CannotDisableOrHealAccount(t *testing.T) {
	account := &Account{ID: 2165, Platform: PlatformGrok, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "test", "base_url": "https://api.x.ai/v1"}}
	repo := &grokQuotaAccountRepoStub{account: account}
	upstream := &grokQuotaHTTPUpstreamStub{response: &http.Response{
		StatusCode: http.StatusForbidden,
		Header:     http.Header{"X-Ratelimit-Limit-Requests": {"100"}},
		Body:       io.NopCloser(strings.NewReader(cloudflare1010Body)),
	}}
	svc := &GrokQuotaService{accountRepo: repo, httpUpstream: upstream}
	snapshot, status, err := svc.probeRateLimitHeaders(context.Background(), account)
	require.Error(t, err)
	require.Equal(t, http.StatusForbidden, status)
	require.Nil(t, snapshot, "an edge block is not an authoritative account quota snapshot")
	require.Zero(t, repo.setErrorCalls)
	require.Zero(t, repo.clearErrorCalls)
	require.Nil(t, repo.setSchedulableTo)
}

func TestCloudflareMarkerDoesNotHideUnauthorizedCredentials(t *testing.T) {
	account := &Account{ID: 2165, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Schedulable: true}
	body := []byte(cloudflare1010Body)
	require.Equal(t, APIKeyStatusActionPermanentDisable, ClassifyAPIKeyStatusAction(account, 401, body))
	repo := &grokQuotaAccountRepoStub{account: account}
	require.True(t, (&RateLimitService{accountRepo: repo}).HandleUpstreamError(context.Background(), account, 401, nil, body))
	require.Positive(t, repo.setErrorCalls)
}

func TestOrdinaryForbiddenStillDisablesOAuthAccount(t *testing.T) {
	account := &Account{ID: 2165, Platform: PlatformAnthropic, Type: AccountTypeOAuth, Schedulable: true}
	repo := &grokQuotaAccountRepoStub{account: account}
	require.True(t, (&RateLimitService{accountRepo: repo}).HandleUpstreamError(context.Background(), account, 403, nil, []byte(`{"error":{"code":1010,"message":"Permission denied"}}`)))
	require.Positive(t, repo.setErrorCalls)
}

func TestCloudflareBlockStillReturnsRequestError(t *testing.T) {
	account := &Account{ID: 2165, Platform: PlatformGrok, Type: AccountTypeAPIKey, Schedulable: true}
	repo := &grokQuotaAccountRepoStub{account: account}
	svc := &OpenAIGatewayService{accountRepo: repo, rateLimitService: &RateLimitService{accountRepo: repo}}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	resp := &http.Response{StatusCode: 403, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(cloudflare1010Body))}
	result, err := svc.handleChatCompletionsErrorResponse(resp, c, account, "grok-4.7")
	require.Error(t, err)
	require.Nil(t, result)
	require.GreaterOrEqual(t, rec.Code, http.StatusBadRequest)
	require.Contains(t, rec.Body.String(), `"error"`)
	require.Zero(t, repo.setErrorCalls)
	require.Nil(t, repo.tempUnschedUntil)
}
