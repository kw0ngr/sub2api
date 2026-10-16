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

const grokContentRefusalBody = `{"code":"permission-denied","error":"I'm sorry, I can't help with that request."}`

func TestIsGrokContentPolicyRejection(t *testing.T) {
	for _, tt := range []struct {
		name     string
		platform string
		status   int
		body     string
		want     bool
	}{
		{"content refusal", PlatformGrok, 403, grokContentRefusalBody, true},
		{"invalid credential", PlatformGrok, 401, grokContentRefusalBody, false},
		{"chat permission", PlatformGrok, 403, `{"code":"permission-denied","error":"Access to the chat endpoint is denied."}`, false},
		{"billing", PlatformGrok, 403, `{"code":"personal-team-blocked:spending-limit","error":"No credits remaining"}`, false},
		{"wrong code", PlatformGrok, 403, strings.ReplaceAll(grokContentRefusalBody, "permission-denied", "invalid_api_key"), false},
		{"other platform", PlatformOpenAI, 403, grokContentRefusalBody, false},
		{"invalid JSON", PlatformGrok, 403, grokContentRefusalBody + "garbage", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			account := &Account{Platform: tt.platform, Type: AccountTypeAPIKey}
			require.Equal(t, tt.want, isGrokContentPolicyRejection(account, tt.status, []byte(tt.body)))
		})
	}
	require.False(t, isGrokContentPolicyRejection(nil, 403, []byte(grokContentRefusalBody)))
	require.Equal(t, APIKeyStatusActionIgnore, ClassifyAPIKeyStatusAction(
		&Account{Platform: PlatformGrok, Type: AccountTypeAPIKey}, 403, []byte(grokContentRefusalBody)))
}

func TestGrokContentRefusalDoesNotMutateHealth(t *testing.T) {
	for _, accountType := range []string{AccountTypeOAuth, AccountTypeAPIKey} {
		t.Run(accountType, func(t *testing.T) {
			account := &Account{ID: 2165, Platform: PlatformGrok, Type: accountType, Schedulable: true,
				Credentials: map[string]any{
					"custom_error_codes_enabled": true,
					"custom_error_codes":         []any{float64(403)},
					"temp_unschedulable_enabled": true,
					"temp_unschedulable_rules": []any{map[string]any{
						"error_code": float64(403), "keywords": []any{"permission-denied"}, "duration_minutes": float64(30),
					}},
				}}
			repo := &grokQuotaAccountRepoStub{account: account}
			svc := &RateLimitService{accountRepo: repo}
			body := []byte(grokContentRefusalBody)
			require.Len(t, account.GetTempUnschedulableRules(), 1)
			require.False(t, svc.HandleUpstreamError(context.Background(), account, 403, nil, body, "grok-4.7"))
			require.Equal(t, ErrorPolicyNone, svc.CheckErrorPolicy(context.Background(), account, 403, body, "grok-4.7"))
			require.False(t, svc.HandleTempUnschedulable(context.Background(), account, 403, body, "grok-4.7"))
			gateway := &OpenAIGatewayService{accountRepo: repo}
			gateway.handleGrokAccountUpstreamError(context.Background(), account, 403, nil, body)
			(&GrokQuotaService{accountRepo: repo}).applyInferenceProbeSideEffects(context.Background(), account, nil, 403, nil, body)
			require.True(t, account.Schedulable)
			require.Zero(t, repo.setErrorCalls)
			require.Zero(t, repo.setSchedulableN)
			require.Zero(t, repo.modelRateLimitN)
			require.Nil(t, repo.tempUnschedUntil)
			require.Nil(t, repo.rateLimitedUntil)
		})
	}
}

func TestGrokContentRefusalIsReturnedWithoutFailover(t *testing.T) {
	gin.SetMode(gin.TestMode)
	account := &Account{ID: 2165, Platform: PlatformGrok, Type: AccountTypeAPIKey, Schedulable: true}
	for _, path := range []string{"chat", "responses", "messages", "passthrough", "media"} {
		t.Run(path, func(t *testing.T) {
			repo := &grokQuotaAccountRepoStub{account: account}
			svc := &OpenAIGatewayService{accountRepo: repo, rateLimitService: &RateLimitService{accountRepo: repo}}
			body := []byte(grokContentRefusalBody)
			require.False(t, svc.shouldFailoverOpenAIUpstreamResponseForAccount(account, 403, "", body, "grok-4.7"))
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
			resp := &http.Response{StatusCode: 403, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(grokContentRefusalBody))}
			var err error
			switch path {
			case "chat":
				_, err = svc.handleChatCompletionsErrorResponse(resp, c, account, "grok-4.7")
			case "responses":
				_, err = svc.handleErrorResponse(context.Background(), resp, c, account, nil, "grok-4.7")
			case "messages":
				_, err = svc.handleAnthropicErrorResponse(resp, c, account, "grok-4.7")
			case "passthrough":
				err = svc.handleFailoverErrorResponsePassthrough(context.Background(), resp, c, account, nil)
			case "media":
				_, err = svc.handleGrokMediaErrorResponse(context.Background(), resp, c, account, "", "grok-imagine")
			}
			require.Error(t, err)
			var failover *UpstreamFailoverError
			require.NotErrorAs(t, err, &failover)
			require.Equal(t, http.StatusForbidden, recorder.Code)
			if path == "chat" || path == "responses" || path == "messages" {
				require.Contains(t, recorder.Body.String(), "content_policy_violation")
			}
			require.Zero(t, repo.setErrorCalls)
			require.Zero(t, repo.modelRateLimitN)
			require.Nil(t, repo.tempUnschedUntil)
		})
	}
}
