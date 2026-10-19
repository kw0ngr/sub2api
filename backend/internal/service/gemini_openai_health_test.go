//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGeminiOpenAIArrayErrorsKeepModelQuotaScoped(t *testing.T) {
	body := []byte(`[{"error":{"code":429,"status":"RESOURCE_EXHAUSTED","message":"Quota exceeded. Please retry in 20s.","details":[{"@type":"type.googleapis.com/google.rpc.QuotaFailure","violations":[{"quotaDimensions":{"model":"gemini-3.1-pro"}}]},{"@type":"type.googleapis.com/google.rpc.RetryInfo","retryDelay":"20s"}]}}]`)
	repo := &rateLimitAccountRepoStub{}
	svc := &RateLimitService{accountRepo: repo}
	account := &Account{ID: 1796, Platform: PlatformGemini, Type: AccountTypeAPIKey}
	require.Equal(t, "Quota exceeded. Please retry in 20s.", extractUpstreamErrorMessage(body))
	require.True(t, svc.HandleUpstreamError(context.Background(), account, 429, nil, body, "gemini-3.1-pro-preview"))
	require.Zero(t, repo.tempCalls)
	require.Zero(t, repo.rateLimitedCalls)
	require.Equal(t, []string{"gemini-3.1-pro-preview"}, repo.modelRateLimitScopes)
	require.WithinDuration(t, time.Now().Add(20*time.Second), repo.modelRateLimitResets[0], 2*time.Second)
}

func TestGeminiOpenAIHighDemandOnlyBrieflyCoolsRequestedModel(t *testing.T) {
	body := []byte(`[{"error":{"code":503,"status":"UNAVAILABLE","message":"This model is currently experiencing high demand. Spikes in demand are usually temporary. Please try again later."}}]`)
	repo := &rateLimitAccountRepoStub{}
	svc := &RateLimitService{accountRepo: repo}
	account := &Account{ID: 1795, Platform: PlatformGemini, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"model_mapping": map[string]any{"gemini-3.7-flash": "gemini-3.8-flash"}}}
	require.True(t, svc.HandleUpstreamError(context.Background(), account, http.StatusServiceUnavailable, nil, body, "gemini-3.7-flash"))
	require.Zero(t, repo.tempCalls)
	require.Zero(t, repo.setErrorCalls)
	require.Equal(t, []string{"gemini-3.8-flash"}, repo.modelRateLimitScopes)
	require.WithinDuration(t, time.Now().Add(30*time.Second), repo.modelRateLimitResets[0], 2*time.Second)
}

func TestGeminiArrayInvalidCredentialStillDisables(t *testing.T) {
	repo := &rateLimitAccountRepoStub{}
	svc := &RateLimitService{accountRepo: repo}
	account := &Account{ID: 1, Platform: PlatformGemini, Type: AccountTypeAPIKey}
	body := []byte(`[{"error":{"code":400,"status":"INVALID_ARGUMENT","message":"API key not valid. Please pass a valid API key."}}]`)
	require.True(t, svc.HandleUpstreamError(context.Background(), account, 400, nil, body))
	require.Equal(t, 1, repo.setErrorCalls)
}

func TestGeminiArrayRetryInfoWithoutMessageDelay(t *testing.T) {
	body := []byte(`[{"error":{"code":429,"status":"RESOURCE_EXHAUSTED","details":[{"@type":"type.googleapis.com/google.rpc.RetryInfo","retryDelay":"15.1s"}]}}]`)
	reset := ParseGeminiRateLimitResetTime(body)
	require.NotNil(t, reset)
	require.WithinDuration(t, time.Now().Add(16*time.Second), time.Unix(*reset, 0), 2*time.Second)
}

func TestGeminiMissingToolSignatureReportsClientError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	body := `[{"error":{"code":400,"message":"Function call is missing a thought_signature in functionCall parts.","status":"INVALID_ARGUMENT"}}]`
	resp := &http.Response{StatusCode: 400, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}
	repo := &rateLimitAccountRepoStub{}
	svc := &OpenAIGatewayService{rateLimitService: &RateLimitService{accountRepo: repo}}
	_, err := svc.handleChatCompletionsErrorResponse(resp, c, &Account{ID: 1, Platform: PlatformGemini, Type: AccountTypeAPIKey}, "gemini-3.8-flash")
	require.Error(t, err)
	require.Equal(t, 400, recorder.Code)
	require.Contains(t, recorder.Body.String(), "tool_calls[].extra_content.google.thought_signature")
	require.Zero(t, repo.setErrorCalls)
	require.Zero(t, repo.tempCalls)
	require.Zero(t, repo.rateLimitedCalls)
}

func TestGeminiShortInvalidAPIKeyDisablesAndFailsOver(t *testing.T) {
	for _, body := range []string{
		`{"error":{"code":400,"message":"Please pass a valid API key","status":"INVALID_ARGUMENT"}}`,
		`[{"error":{"code":400,"message":"Please pass a valid API key","status":"INVALID_ARGUMENT"}}]`,
		`[{"error":{"code":400,"message":"Please pass a valid API key.","status":"INVALID_ARGUMENT"}}]`,
		`{"error":{"code":400,"message":"API key not valid. Please pass a valid API key.","status":"INVALID_ARGUMENT"}}`,
	} {
		t.Run(body, func(t *testing.T) {
			account := &Account{ID: 1808, Platform: PlatformGemini, Type: AccountTypeAPIKey, Schedulable: true}
			repo := &grokQuotaAccountRepoStub{account: account}
			svc := &OpenAIGatewayService{rateLimitService: &RateLimitService{accountRepo: repo}}
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
			response := &http.Response{StatusCode: http.StatusBadRequest, Header: http.Header{}}

			require.Equal(t, APIKeyStatusActionPermanentDisable, ClassifyAPIKeyStatusAction(account, 400, []byte(body)))
			valid, invalid, cooldown, _ := ClassifyAPIKeyProbeResponse(account, 400, []byte(body))
			require.False(t, valid)
			require.True(t, invalid)
			require.False(t, cooldown)
			failover := svc.failoverOpenAIUpstreamHTTPError(context.Background(), c, account, response, []byte(body), extractUpstreamErrorMessage([]byte(body)), "gemini-3.8-flash")
			require.NotNil(t, failover)
			require.Equal(t, 400, failover.StatusCode)
			require.False(t, c.Writer.Written(), "do not commit a response before trying the next account")
			require.Equal(t, 1, repo.setErrorCalls)
			require.Equal(t, 1, repo.setSchedulableN)
			require.False(t, account.Schedulable)
			require.Equal(t, StatusError, account.Status)
			require.Nil(t, repo.tempUnschedUntil)
		})
	}
}

func TestGeminiRequest400DoesNotFailOverOrDisable(t *testing.T) {
	account := &Account{Platform: PlatformGemini, Type: AccountTypeAPIKey}
	svc := &OpenAIGatewayService{}
	for _, body := range []string{
		`[{"error":{"code":400,"message":"Function call is missing a thought_signature in functionCall parts.","status":"INVALID_ARGUMENT"}}]`,
		`[{"error":{"code":400,"message":"Invalid value for generation_config","status":"INVALID_ARGUMENT"}}]`,
		`[{"error":{"code":400,"message":"Tool description contains 'Please pass a valid API key'","status":"INVALID_ARGUMENT"}}]`,
	} {
		require.Equal(t, APIKeyStatusActionIgnore, ClassifyAPIKeyStatusAction(account, 400, []byte(body)))
		require.False(t, svc.shouldFailoverOpenAIUpstreamResponseForAccount(account, 400, extractUpstreamErrorMessage([]byte(body)), []byte(body), "gemini-3.8-flash"))
	}
	require.False(t, svc.shouldFailoverOpenAIUpstreamResponseForAccount(
		&Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}, 400, "",
		[]byte(`{"error":{"message":"Please pass a valid API key"}}`), "gpt-5.6-sol",
	))
}

func TestGeminiInvalidKeyForwardingLeavesResponseAvailableForNextAccount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, protocol := range []string{"chat", "responses", "messages"} {
		t.Run(protocol, func(t *testing.T) {
			account := &Account{ID: 1808, Platform: PlatformGemini, Type: AccountTypeAPIKey, Schedulable: true, Concurrency: 1, Credentials: map[string]any{"api_key": "test-invalid"}}
			repo := &grokQuotaAccountRepoStub{account: account}
			upstream := &httpUpstreamRecorder{resp: &http.Response{
				StatusCode: 400, Header: http.Header{"Content-Type": []string{"application/json"}},
				Body: io.NopCloser(strings.NewReader(`[{"error":{"code":400,"message":"Please pass a valid API key","status":"INVALID_ARGUMENT"}}]`)),
			}}
			svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream, rateLimitService: &RateLimitService{accountRepo: repo}}
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/"+protocol, nil)
			var err error
			switch protocol {
			case "chat":
				_, err = svc.ForwardAsChatCompletions(context.Background(), c, account,
					[]byte(`{"model":"gemini-3.8-flash","messages":[{"role":"user","content":"hi"}]}`), "", "")
			case "responses":
				_, err = svc.Forward(context.Background(), c, account,
					[]byte(`{"model":"gemini-3.8-flash","input":"hi"}`))
			case "messages":
				_, err = svc.ForwardAsAnthropic(context.Background(), c, account,
					[]byte(`{"model":"gemini-3.8-flash","max_tokens":64,"messages":[{"role":"user","content":"hi"}]}`), "", "")
			}
			var failover *UpstreamFailoverError
			require.ErrorAs(t, err, &failover)
			require.Equal(t, 400, failover.StatusCode)
			require.False(t, c.Writer.Written())
			require.Equal(t, StatusError, account.Status)
			require.False(t, account.Schedulable)
		})
	}
}
