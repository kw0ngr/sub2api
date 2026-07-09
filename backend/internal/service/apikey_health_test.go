//go:build unit

package service

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDetectAPIKeyPlatform(t *testing.T) {
	tests := []struct {
		key      string
		platform string
		ok       bool
	}{
		{key: "sk-ant-api03-abc", platform: PlatformAnthropic, ok: true},
		{key: "AIzaSyD-example", platform: PlatformGemini, ok: true},
		{key: "sk-or-v1-example", platform: PlatformOpenRouter, ok: true},
		{key: "sk-proj-123", platform: PlatformOpenAI, ok: true},
		{key: "unknown-key", platform: "", ok: false},
	}

	for _, tt := range tests {
		platform, ok := DetectAPIKeyPlatform(tt.key)
		require.Equal(t, tt.platform, platform)
		require.Equal(t, tt.ok, ok)
	}
}

func TestClassifyAPIKeyStatusAction(t *testing.T) {
	openAI := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
	anthropic := &Account{Platform: PlatformAnthropic, Type: AccountTypeAPIKey}
	gemini := &Account{Platform: PlatformGemini, Type: AccountTypeAPIKey}
	openRouter := &Account{Platform: PlatformOpenRouter, Type: AccountTypeAPIKey}
	deepSeek := &Account{Platform: PlatformDeepSeek, Type: AccountTypeAPIKey}

	require.Equal(t, APIKeyStatusActionValid, ClassifyAPIKeyStatusAction(openAI, http.StatusOK, []byte(`{}`)))
	require.Equal(t, APIKeyStatusActionPermanentDisable, ClassifyAPIKeyStatusAction(openAI, http.StatusForbidden, []byte(`{"error":{"message":"organization has been disabled","code":"account_deactivated"}}`)))
	require.Equal(t, APIKeyStatusActionTemporaryCooldown, ClassifyAPIKeyStatusAction(openAI, http.StatusForbidden, []byte(`{"error":{"message":"model not allowed for this project","code":"forbidden"}}`)))
	require.Equal(t, APIKeyStatusActionTemporaryCooldown, ClassifyAPIKeyStatusAction(anthropic, http.StatusMethodNotAllowed, []byte(`method not allowed`)))
	require.Equal(t, APIKeyStatusActionTemporaryCooldown, ClassifyAPIKeyStatusAction(gemini, http.StatusTooManyRequests, []byte(`{"error":{"message":"quota exceeded"}}`)))
	require.Equal(t, APIKeyStatusActionPermanentDisable, ClassifyAPIKeyStatusAction(gemini, http.StatusBadRequest, []byte(`{"error":{"message":"API key not valid. Please pass a valid API key.","status":"API_KEY_INVALID"}}`)))
	require.Equal(t, APIKeyStatusActionPermanentDisable, ClassifyAPIKeyStatusAction(openRouter, http.StatusUnauthorized, []byte(`{"error":{"message":"invalid token"}}`)))
	require.Equal(t, APIKeyStatusActionTemporaryCooldown, ClassifyAPIKeyStatusAction(deepSeek, http.StatusTooManyRequests, []byte(`{"error":{"message":"rate limited"}}`)))
}

func TestClassifyAPIKeyStatusAction_ParameterValidationIgnored(t *testing.T) {
	account := &Account{Platform: PlatformGLM, Type: AccountTypeAPIKey}
	body := []byte(`{"error":{"message":"The parameter ` + "`max_tokens`" + ` specified in the request is not valid: integer above maximum value, expected a value <= 128000, but got 131072 instead."}}`)

	require.Equal(t, APIKeyStatusActionIgnore, ClassifyAPIKeyStatusAction(account, http.StatusBadRequest, body))
}

func TestClassifyAPIKeyStatusAction_GLMRedactedThinkingValidationIgnored(t *testing.T) {
	account := &Account{Platform: PlatformGLM, Type: AccountTypeAPIKey}
	body := []byte(`{"error":{"code":"InvalidParameter","message":"The parameter ` + "`messages.content.type`" + ` specified in the request are not valid: invalid value: ` + "`redacted_thinking`" + `, supported values are: ` + "`text`" + `, ` + "`thinking`" + `, ` + "`image`" + `, 'tool_use' and ` + "`tool_result`" + `.","type":"BadRequest"}}`)

	require.Equal(t, APIKeyStatusActionIgnore, ClassifyAPIKeyStatusAction(account, http.StatusBadRequest, body))
}

func TestClassifyAPIKeyStatusAction_AnthropicToolUseInputValidationIgnored(t *testing.T) {
	account := &Account{Platform: PlatformAnthropic, Type: AccountTypeAPIKey}
	body := []byte(`{"type":"error","error":{"type":"invalid_request_error","message":"messages.863.content.1.tool_use.input: Input should be an object"}}`)

	require.Equal(t, APIKeyStatusActionIgnore, ClassifyAPIKeyStatusAction(account, http.StatusBadRequest, body))
}

func TestClassifyAPIKeyStatusAction_AnthropicUnsupportedEffortValidationIgnored(t *testing.T) {
	account := &Account{Platform: PlatformAnthropic, Type: AccountTypeAPIKey}
	body := []byte(`{"type":"error","error":{"type":"invalid_request_error","message":"This model does not support effort level 'xhigh'. Supported levels: high, low, max, medium."}}`)

	require.Equal(t, APIKeyStatusActionIgnore, ClassifyAPIKeyStatusAction(account, http.StatusBadRequest, body))
}

func TestClassifyAPIKeyStatusAction_DeepSeekThinkingToolChoiceValidationIgnored(t *testing.T) {
	account := &Account{Platform: PlatformDeepSeek, Type: AccountTypeAPIKey}
	body := []byte(`{"error":{"message":"Thinking mode does not support this tool_choice","type":"invalid_request_error"}}`)

	require.Equal(t, APIKeyStatusActionIgnore, ClassifyAPIKeyStatusAction(account, http.StatusBadRequest, body))
}

func TestClassifyAPIKeyStatusAction_OpenAIImageGenerationItemIDValidationIgnored(t *testing.T) {
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
	body := []byte(`{"error":{"message":"Image generation items without ` + "`id`" + ` are not supported for this request.","type":"invalid_request_error"}}`)

	require.Equal(t, APIKeyStatusActionIgnore, ClassifyAPIKeyStatusAction(account, http.StatusBadRequest, body))
}

func TestClassifyAPIKeyStatusAction_GLMResettableQuotaIsTemporaryCooldown(t *testing.T) {
	// Given
	account := &Account{Platform: PlatformGLM, Type: AccountTypeAPIKey}
	body := []byte(`{"error":{"code":"1310","message":"[1310][您已达到每周/每月使用上限，您的限额将在 2026-06-26 15:46:24 重置。][20260621010506]","type":"rate_limit_error"},"retry-after":"484876"}`)

	// When
	action := ClassifyAPIKeyStatusAction(account, http.StatusForbidden, body)

	// Then
	require.Equal(t, APIKeyStatusActionTemporaryCooldown, action)
}

func TestClassifyAPIKeyStatusAction_OpenAIContentPolicyServerErrorIgnored(t *testing.T) {
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}

	tests := []struct {
		name       string
		statusCode int
		body       []byte
	}{
		{
			name:       "message flagged",
			statusCode: http.StatusBadGateway,
			body: []byte(`{
				"error": {
					"message": "This content was flagged for possible cybersecurity risk. If this seems wrong, try rephrasing your request. To get authorized for security work, join the Trusted Access for Cyber program: https://chatgpt.com/cyber",
					"type": "server_error"
				}
			}`),
		},
		{
			name:       "structured cyber policy code",
			statusCode: http.StatusBadRequest,
			body:       []byte(`{"error":{"message":"blocked","code":"cyber_policy","type":"invalid_request_error"}}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, APIKeyStatusActionIgnore, ClassifyAPIKeyStatusAction(account, tt.statusCode, tt.body))
		})
	}
}

func TestClassifyAPIKeyProbeResponse(t *testing.T) {
	openAIAccount := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
	geminiAccount := &Account{Platform: PlatformGemini, Type: AccountTypeAPIKey}
	anthropicAccount := &Account{Platform: PlatformAnthropic, Type: AccountTypeAPIKey}
	openRouterAccount := &Account{Platform: PlatformOpenRouter, Type: AccountTypeAPIKey}

	valid, invalid, cooldown, _ := ClassifyAPIKeyProbeResponse(openAIAccount, http.StatusOK, []byte(`{}`))
	require.True(t, valid)
	require.False(t, invalid)
	require.False(t, cooldown)

	valid, invalid, cooldown, _ = ClassifyAPIKeyProbeResponse(openAIAccount, http.StatusPaymentRequired, []byte(`{"error":{"message":"insufficient balance"}}`))
	require.False(t, valid)
	require.True(t, invalid)
	require.False(t, cooldown)

	valid, invalid, cooldown, _ = ClassifyAPIKeyProbeResponse(geminiAccount, http.StatusBadRequest, []byte(`{"error":{"message":"API key not valid. Please pass a valid API key.","status":"API_KEY_INVALID"}}`))
	require.False(t, valid)
	require.True(t, invalid)
	require.False(t, cooldown)

	// OpenAI 403 unrecognized → cooldown (not permanent disable, not valid)
	valid, invalid, cooldown, _ = ClassifyAPIKeyProbeResponse(openAIAccount, http.StatusForbidden, []byte(`{"error":{"message":"model not allowed for this project","code":"forbidden"}}`))
	require.False(t, valid)
	require.False(t, invalid)
	require.True(t, cooldown)

	// Unknown status code (405) → cooldown
	valid, invalid, cooldown, _ = ClassifyAPIKeyProbeResponse(anthropicAccount, http.StatusMethodNotAllowed, []byte(`method not allowed`))
	require.False(t, valid)
	require.False(t, invalid)
	require.True(t, cooldown)

	valid, invalid, cooldown, _ = ClassifyAPIKeyProbeResponse(openAIAccount, http.StatusTooManyRequests, []byte(`{"error":{"message":"rate limited"}}`))
	require.False(t, valid)
	require.False(t, invalid)
	require.True(t, cooldown)

	valid, invalid, cooldown, _ = ClassifyAPIKeyProbeResponse(openRouterAccount, http.StatusOK, []byte(`{}`))
	require.True(t, valid)
	require.False(t, invalid)
	require.False(t, cooldown)
}

func TestDefaultAPIKeyBaseURL_OpenRouterAndDeepSeek(t *testing.T) {
	require.Equal(t, "https://openrouter.ai/api/v1", DefaultAPIKeyBaseURL(PlatformOpenRouter))
	require.Equal(t, "https://api.deepseek.com", DefaultAPIKeyBaseURL(PlatformDeepSeek))
	require.Equal(t, "https://api.x.ai/v1", DefaultAPIKeyBaseURL(PlatformGrok))
}

func TestCheckOpenRouterAPIKey_LowBalanceIsInvalid(t *testing.T) {
	svc := &AccountTestService{
		httpUpstream: &queuedHTTPUpstream{responses: []*http.Response{
			newJSONResponse(http.StatusOK, `{"data":{"total_credits":1.2,"total_usage":0.5}}`),
		}},
	}
	account := &Account{
		ID:          1,
		Platform:    PlatformOpenRouter,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{"api_key": "sk-or-v1-test"},
	}

	health, err := svc.CheckAPIKeyValidity(context.Background(), account)

	require.NoError(t, err)
	require.False(t, health.Valid)
	require.True(t, health.Invalid)
	require.Equal(t, http.StatusOK, health.StatusCode)
	require.NotNil(t, health.ProbeQuota)
	require.Equal(t, "0.7000", health.ProbeQuota.CreditsRemaining)
	require.Contains(t, health.Message, "below 1")
}

func TestCheckDeepSeekAPIKey_LowBalanceIsInvalid(t *testing.T) {
	svc := &AccountTestService{
		httpUpstream: &queuedHTTPUpstream{responses: []*http.Response{
			newJSONResponse(http.StatusOK, `{"balance_infos":[{"total_balance":"0.50","currency":"USD"}]}`),
		}},
	}
	account := &Account{
		ID:          2,
		Platform:    PlatformDeepSeek,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{"api_key": "sk-test"},
	}

	health, err := svc.CheckAPIKeyValidity(context.Background(), account)

	require.NoError(t, err)
	require.False(t, health.Valid)
	require.True(t, health.Invalid)
	require.Equal(t, http.StatusOK, health.StatusCode)
	require.NotNil(t, health.ProbeQuota)
	require.Equal(t, "0.50 USD", health.ProbeQuota.Balance)
	require.Contains(t, health.Message, "below 1")
}

func TestCheckDeepSeekAPIKey_AnyBalanceAtLeastOneStaysValid(t *testing.T) {
	svc := &AccountTestService{
		httpUpstream: &queuedHTTPUpstream{responses: []*http.Response{
			newJSONResponse(http.StatusOK, `{"balance_infos":[{"total_balance":"0.50","currency":"USD"},{"total_balance":"2.00","currency":"CNY"}]}`),
		}},
	}
	account := &Account{
		ID:          3,
		Platform:    PlatformDeepSeek,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{"api_key": "sk-test"},
	}

	health, err := svc.CheckAPIKeyValidity(context.Background(), account)

	require.NoError(t, err)
	require.True(t, health.Valid)
	require.False(t, health.Invalid)
	require.Equal(t, http.StatusOK, health.StatusCode)
	require.NotNil(t, health.ProbeQuota)
	require.Equal(t, "0.50 USD; 2.00 CNY", health.ProbeQuota.Balance)
}

func TestClassifyAPIKeyStatusAction_OpenRouterPaymentRequiredDisables(t *testing.T) {
	account := &Account{Platform: PlatformOpenRouter, Type: AccountTypeAPIKey}

	action := ClassifyAPIKeyStatusAction(account, http.StatusPaymentRequired, []byte(`{"error":{"message":"Insufficient credits"}}`))

	require.Equal(t, APIKeyStatusActionPermanentDisable, action)
}

func TestClassifyAPIKeyStatusAction_DeepSeekInsufficientBalanceDisables(t *testing.T) {
	account := &Account{Platform: PlatformDeepSeek, Type: AccountTypeAPIKey}

	action := ClassifyAPIKeyStatusAction(account, http.StatusBadRequest, []byte(`{"error":{"message":"Insufficient balance"}}`))

	require.Equal(t, APIKeyStatusActionPermanentDisable, action)
}

func TestBuildAPIKeyHealthCheckResultFromScheduledResult_InvalidFailure(t *testing.T) {
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
	result := &ScheduledTestResult{
		Status:       "failed",
		ErrorMessage: `API returned 401: {"error":{"message":"invalid api key","code":"invalid_api_key"}}`,
	}

	health := buildAPIKeyHealthCheckResultFromScheduledResult(account, result)

	require.False(t, health.Valid)
	require.True(t, health.Invalid)
	require.Equal(t, http.StatusUnauthorized, health.StatusCode)
	require.Contains(t, health.Message, "invalid api key")
}

func TestBuildAPIKeyHealthCheckResultFromScheduledResult_CooldownFailure(t *testing.T) {
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
	result := &ScheduledTestResult{
		Status:       "failed",
		ErrorMessage: `API returned 429: {"error":{"message":"rate limited","code":"rate_limit_exceeded"}}`,
	}

	health := buildAPIKeyHealthCheckResultFromScheduledResult(account, result)

	require.False(t, health.Valid)
	require.False(t, health.Invalid)
	require.Equal(t, http.StatusTooManyRequests, health.StatusCode)
	require.Contains(t, health.Message, "rate limited")
}
