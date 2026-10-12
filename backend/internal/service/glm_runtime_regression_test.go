//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestGLM1113StopsOnlySelectedEndpointInsteadOfRepeatingCooldown(t *testing.T) {
	body := []byte(`{"error":{"code":"1113","message":"余额不足或无可用资源包,请充值。"}}`)
	for _, mode := range []string{GLMCompatModeOpenAI, GLMCompatModeAnthropic} {
		repo := &rateLimitAccountRepoStubWithSchedulable{}
		svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
		account := &Account{ID: 2159, Platform: PlatformGLM, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true,
			Credentials: map[string]any{"api_key": "same-upstream-key", "compat_mode": mode}}
		require.Equal(t, APIKeyStatusActionPermanentDisable, ClassifyAPIKeyStatusAction(account, http.StatusTooManyRequests, body))
		require.True(t, svc.HandleUpstreamError(context.Background(), account, http.StatusTooManyRequests, nil, body, "glm-5.3"))
		require.Equal(t, 1, repo.setErrorCalls)
		require.Equal(t, 1, repo.setSchedulableCalls)
		require.False(t, repo.lastSchedulable)
		require.Zero(t, repo.rateLimitedCalls)
		require.Zero(t, repo.tempCalls)
		require.Contains(t, repo.lastErrorMsg, "1113")
	}
	account := &Account{Platform: PlatformGLM, Type: AccountTypeAPIKey}
	require.Equal(t, APIKeyStatusActionTemporaryCooldown, ClassifyAPIKeyStatusAction(account, 429,
		[]byte(`{"error":{"code":"1310","message":"使用上限，限额将重置"}}`)))
	require.Equal(t, APIKeyStatusActionTemporaryCooldown, ClassifyAPIKeyStatusAction(account, 429,
		[]byte(`{"error":{"message":"Too many concurrent requests"}}`)))
	require.Equal(t, APIKeyStatusActionIgnore, ClassifyAPIKeyStatusAction(account, 400,
		[]byte(`{"error":{"code":"1301","message":"content rejected"}}`)))
}

func TestGLMHealthModels200CannotRestoreEndpointWithNoInferenceResources(t *testing.T) {
	upstream := &glmProbeHTTPUpstream{responses: []*http.Response{
		{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"data":[{"id":"glm-5.3-flash"}]}`))},
		{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"code":200,"data":{"limits":[]}}`))},
		{StatusCode: 429, Body: io.NopCloser(strings.NewReader(`{"error":{"code":"1113","message":"余额不足或无可用资源包"}}`))},
	}}
	svc := &AccountTestService{httpUpstream: upstream}
	account := &Account{ID: 2159, Platform: PlatformGLM, Type: AccountTypeAPIKey, Status: StatusError, Schedulable: false,
		Credentials: map[string]any{
			"api_key": "sk-test", "compat_mode": GLMCompatModeOpenAI, "base_url": "https://open.bigmodel.cn/api/paas/v4",
			"model_mapping": map[string]any{"glm-5.3-flash": "glm-5.3-flash"},
		}}
	list, err := svc.checkAPIKeyModelsEndpoint(context.Background(), account, PlatformGLM, "https://open.bigmodel.cn/api/paas/v4/models")
	require.NoError(t, err)
	require.True(t, list.Valid, "model inventory can be available while inference resources are exhausted")
	health, err := svc.CheckAPIKeyValidity(context.Background(), account)
	require.NoError(t, err)
	require.False(t, health.Valid)
	require.True(t, health.Invalid)
	require.Equal(t, 429, health.StatusCode)
	require.Len(t, upstream.requests, 3)
	require.Equal(t, http.MethodPost, upstream.requests[2].Method)
	require.Equal(t, "glm-5.3-flash", gjson.GetBytes(upstream.bodies[2], "model").String())
	require.Equal(t, "low", gjson.GetBytes(upstream.bodies[2], "reasoning_effort").String())
}

func TestGLMHealthRequiresActualTextNotJustHTTP200(t *testing.T) {
	upstream := &glmProbeHTTPUpstream{responses: []*http.Response{
		{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":""}}]}`))},
	}}
	svc := &AccountTestService{httpUpstream: upstream}
	account := &Account{Platform: PlatformGLM, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "sk-test"}}
	health, err := svc.checkAPIKeyChatProbe(context.Background(), account, "https://open.bigmodel.cn/api/paas/v4", glmAPIKeyProbeModel(account))
	require.NoError(t, err)
	require.False(t, health.Valid)
	require.False(t, health.Invalid, "empty output is not proof of an invalid credential")
	upstream.responses = []*http.Response{{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"content":[{"type":"thinking","thinking":"working"}]}`))}}
	health, err = svc.checkGLMAnthropicMessagesProbe(context.Background(), account, "https://open.bigmodel.cn/api/anthropic")
	require.NoError(t, err)
	require.False(t, health.Valid, "thinking without a final response is not sufficient to restore scheduling")
}

func TestGLMFlashLowReasoningIsNotSilentlyUpgraded(t *testing.T) {
	body := []byte(`{"model":"glm-5.3-flash","reasoning_effort":"low"}`)
	out, changed := NormalizeGLMOpenAIReasoningEffort(body, "glm-5.3-flash")
	require.False(t, changed)
	require.Equal(t, body, out)
}

func TestChatCompletionChunksProduceVisibleFirstToken(t *testing.T) {
	for _, payload := range []string{
		`{"object":"chat.completion.chunk","choices":[{"delta":{"content":"hi"}}]}`,
		`{"choices":[{"delta":{"reasoning_content":"thinking"}}]}`,
		`{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"name":"exec"}}]}}]}`,
		`{"choices":[{"delta":{"function_call":{"name":"exec"}}}]}`,
		`{"choices":[{"delta":{"content":[{"type":"text","text":"hi"}]}}]}`,
	} {
		require.True(t, openAIStreamDataStartsVisibleOutput(payload, ""), payload)
	}
	for _, payload := range []string{
		`{"choices":[{"delta":{"role":"assistant","content":""}}]}`,
		`{"choices":[],"usage":{"completion_tokens":1}}`,
		`{"choices":[{"delta":{},"finish_reason":"stop"}]}`,
		`{"choices":[{"delta":{"content":[]}}]}`,
		`[DONE]`,
	} {
		require.False(t, openAIStreamDataStartsVisibleOutput(payload, ""), payload)
	}
}

func TestGLM53ManualTestsUseSupportedLowThinking(t *testing.T) {
	for _, model := range []string{"glm-5.3", "glm-5.3-flash"} {
		upstream := &glmProbeHTTPUpstream{responses: []*http.Response{{
			StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"successful"}}]}`)),
		}}}
		svc := &AccountTestService{httpUpstream: upstream, cfg: &config.Config{}}
		account := &Account{ID: 2159, Platform: PlatformGLM, Type: AccountTypeAPIKey,
			Credentials: map[string]any{"api_key": "sk-test", "base_url": "https://open.bigmodel.cn/api/coding/paas/v4", "compat_mode": GLMCompatModeOpenAI}}
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/test", nil)
		require.NoError(t, svc.testGLMOpenAIAccountConnection(c, account, model, ""))
		require.Equal(t, "enabled", gjson.GetBytes(upstream.bodies[0], "thinking.type").String())
		require.Equal(t, "low", gjson.GetBytes(upstream.bodies[0], "reasoning_effort").String())
	}
}
