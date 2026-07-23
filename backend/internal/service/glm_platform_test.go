package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestGLMDefaultBaseURLs(t *testing.T) {
	require.Equal(t, "https://open.bigmodel.cn/api/anthropic", DefaultGLMAnthropicBaseURL())
	require.Equal(t, "https://open.bigmodel.cn/api/paas/v4", DefaultAPIKeyBaseURL("glm"))
}

func TestBuildGLMOpenAICompatibleChatCompletionsURL(t *testing.T) {
	tests := []struct {
		name string
		base string
		want string
	}{
		{
			name: "official general base appends chat completions without v1",
			base: "https://open.bigmodel.cn/api/paas/v4",
			want: "https://open.bigmodel.cn/api/paas/v4/chat/completions",
		},
		{
			name: "official coding base appends chat completions without v1",
			base: "https://open.bigmodel.cn/api/coding/paas/v4",
			want: "https://open.bigmodel.cn/api/coding/paas/v4/chat/completions",
		},
		{
			name: "generic relay v1 base keeps openai compatible shape",
			base: "https://relay.example.test/v1",
			want: "https://relay.example.test/v1/chat/completions",
		},
		{
			name: "complete chat completions URL stays unchanged",
			base: "https://relay.example.test/custom/chat/completions",
			want: "https://relay.example.test/custom/chat/completions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, buildOpenAICompatibleChatCompletionsURL("glm", tt.base))
		})
	}
}

func TestBuildGLMAnthropicMessagesURL(t *testing.T) {
	tests := []struct {
		name string
		base string
		want string
	}{
		{
			name: "official anthropic base appends v1 messages",
			base: "https://open.bigmodel.cn/api/anthropic",
			want: "https://open.bigmodel.cn/api/anthropic/v1/messages",
		},
		{
			name: "generic relay v1 base appends messages",
			base: "https://relay.example.test/v1",
			want: "https://relay.example.test/v1/messages",
		},
		{
			name: "complete messages URL stays unchanged",
			base: "https://relay.example.test/custom/v1/messages",
			want: "https://relay.example.test/custom/v1/messages",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, buildGLMAnthropicMessagesURL(tt.base))
		})
	}
}

func TestBuildGLMOpenAIModelsURLHandlesCompleteChatURL(t *testing.T) {
	require.Equal(
		t,
		"https://open.bigmodel.cn/api/paas/v4/models",
		buildGLMOpenAIModelsURL("https://open.bigmodel.cn/api/paas/v4/chat/completions"),
	)
	require.Equal(
		t,
		"https://relay.example.test/v1/models",
		buildGLMOpenAIModelsURL("https://relay.example.test/v1/chat/completions"),
	)
}

func TestBuildGLMMonitorQuotaLimitURL(t *testing.T) {
	tests := []struct {
		name string
		base string
		want string
		ok   bool
	}{
		{
			name: "official anthropic base",
			base: "https://open.bigmodel.cn/api/anthropic",
			want: "https://open.bigmodel.cn/api/monitor/usage/quota/limit",
			ok:   true,
		},
		{
			name: "official coding base",
			base: "https://open.bigmodel.cn/api/coding/paas/v4",
			want: "https://open.bigmodel.cn/api/monitor/usage/quota/limit",
			ok:   true,
		},
		{
			name: "zai official base",
			base: "https://api.z.ai/api/anthropic",
			want: "https://api.z.ai/api/monitor/usage/quota/limit",
			ok:   true,
		},
		{
			name: "custom relay",
			base: "https://relay.example.test/v1",
			ok:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := buildGLMMonitorQuotaLimitURL(tt.base)
			require.Equal(t, tt.ok, ok)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestClampGLMMaxTokens(t *testing.T) {
	body := []byte(`{"model":"glm-5.2","max_tokens":131072,"messages":[{"role":"user","content":"hi"}]}`)

	out, clamped := ClampGLMMaxTokens(body)

	require.True(t, clamped)
	require.Equal(t, int64(glmAnthropicMaxTokens), gjson.GetBytes(out, "max_tokens").Int())

	out, clamped = ClampGLMMaxTokens([]byte(`{"model":"glm-5.2","max_tokens":128000}`))
	require.False(t, clamped)
	require.Equal(t, int64(128000), gjson.GetBytes(out, "max_tokens").Int())
}

func TestGLMPlatformHelpers(t *testing.T) {
	require.False(t, IsOpenAICompatiblePlatform("glm"), "GLM should not enable /v1/responses passthrough")
	require.True(t, IsOpenAIChatCompletionsCompatiblePlatform("glm"))
	require.True(t, IsOpenAICompatiblePlatform("grok"))
	require.True(t, IsOpenAIChatCompletionsCompatiblePlatform("grok"))
	require.True(t, SupportedAPIKeyProbePlatform("glm"))
	require.True(t, SupportedAPIKeyProbePlatform("grok"))
	platform, ok := NormalizeAPIKeyPlatform("glm")
	require.True(t, ok)
	require.Equal(t, "glm", platform)
	platform, ok = NormalizeAPIKeyPlatform("x.ai")
	require.True(t, ok)
	require.Equal(t, "grok", platform)
}

func TestAnthropicCompatibleAPIKeyAuthHeaderForGLMOllama(t *testing.T) {
	tests := []struct {
		name              string
		account           *Account
		wantAuthorization string
		wantAPIKey        string
	}{
		{
			name: "ollama hosted glm anthropic mode uses bearer",
			account: &Account{
				Platform: PlatformGLM,
				Type:     AccountTypeAPIKey,
				Credentials: map[string]any{
					"base_url":    "https://ollama.com",
					"compat_mode": GLMCompatModeAnthropic,
				},
			},
			wantAuthorization: "Bearer sk-test",
		},
		{
			name: "official glm anthropic mode keeps x api key",
			account: &Account{
				Platform: PlatformGLM,
				Type:     AccountTypeAPIKey,
				Credentials: map[string]any{
					"base_url":    "https://open.bigmodel.cn/api/anthropic",
					"compat_mode": GLMCompatModeAnthropic,
				},
			},
			wantAPIKey: "sk-test",
		},
		{
			name: "openrouter still uses bearer",
			account: &Account{
				Platform: PlatformOpenRouter,
				Type:     AccountTypeAPIKey,
			},
			wantAuthorization: "Bearer sk-test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header := http.Header{}
			header.Set("Authorization", "Bearer stale")
			header.Set("x-api-key", "stale")
			header.Set("x-goog-api-key", "stale")

			setAnthropicCompatibleAPIKeyAuthHeaderForAccount(header, tt.account, "sk-test")

			require.Equal(t, tt.wantAuthorization, getHeaderRaw(header, "authorization"))
			require.Equal(t, tt.wantAPIKey, getHeaderRaw(header, "x-api-key"))
			require.Empty(t, getHeaderRaw(header, "x-goog-api-key"))
		})
	}
}

func TestGatewayServiceSelectGLMAnthropicModeOnly(t *testing.T) {
	ctx := context.Background()
	groupID := int64(41001)
	accounts := []Account{
		{
			ID:          1,
			Platform:    PlatformGLM,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Priority:    1,
			Credentials: map[string]any{"api_key": "sk-glm-openai", "compat_mode": "openai"},
		},
		{
			ID:          2,
			Platform:    PlatformGLM,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Priority:    2,
			Credentials: map[string]any{"api_key": "sk-glm-anthropic", "compat_mode": "anthropic"},
		},
	}
	svc := &GatewayService{accountRepo: stubOpenAIAccountRepo{accounts: accounts}, cfg: &config.Config{}}

	account, err := svc.selectAccountForModelWithPlatform(ctx, &groupID, "", "glm-5.2", nil, PlatformGLM)

	require.NoError(t, err)
	require.NotNil(t, account)
	require.Equal(t, int64(2), account.ID)
	require.True(t, account.IsGLMAnthropicCompatible())
}

func TestOpenAIGatewayServiceSelectGLMOpenAIModeOnly(t *testing.T) {
	ctx := context.Background()
	groupID := int64(41002)
	accounts := []Account{
		{
			ID:          1,
			Platform:    PlatformGLM,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Priority:    1,
			Credentials: map[string]any{"api_key": "sk-glm-anthropic", "compat_mode": "anthropic"},
		},
		{
			ID:          2,
			Platform:    PlatformGLM,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Priority:    2,
			Credentials: map[string]any{"api_key": "sk-glm-openai", "compat_mode": "openai"},
		},
	}
	svc := &OpenAIGatewayService{
		accountRepo:        stubOpenAIAccountRepo{accounts: accounts},
		cfg:                &config.Config{},
		concurrencyService: NewConcurrencyService(stubConcurrencyCache{}),
	}

	selection, _, err := svc.SelectAccountWithSchedulerForPlatform(
		ctx,
		PlatformGLM,
		&groupID,
		"",
		"",
		"glm-5.2",
		nil,
		OpenAIUpstreamTransportAny,
	)

	require.NoError(t, err)
	require.NotNil(t, selection)
	require.NotNil(t, selection.Account)
	require.Equal(t, int64(2), selection.Account.ID)
	require.True(t, selection.Account.IsGLMOpenAICompatible())
}

func TestOpenAIGatewayServiceForwardGLMOpenAIUsesChatCompletionsEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"glm-req"}},
			Body: io.NopCloser(strings.NewReader(`{
				"id":"chatcmpl_glm",
				"object":"chat.completion",
				"created":1,
				"model":"glm-5.2",
				"choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],
				"usage":{"prompt_tokens":6,"completion_tokens":2,"total_tokens":8}
			}`)),
		},
	}
	svc := &OpenAIGatewayService{httpUpstream: upstream, cfg: &config.Config{}}
	account := &Account{
		ID:          99,
		Platform:    PlatformGLM,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":     "sk-glm-test",
			"base_url":    "https://open.bigmodel.cn/api/paas/v4",
			"compat_mode": "openai",
		},
	}

	result, err := svc.ForwardAsChatCompletions(
		context.Background(),
		c,
		account,
		[]byte(`{"model":"glm-5.2","messages":[{"role":"user","content":"hi"}],"stream":false}`),
		"",
		"",
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, upstream.lastReq)
	require.Equal(t, "https://open.bigmodel.cn/api/paas/v4/chat/completions", upstream.lastReq.URL.String())
	require.Equal(t, "Bearer sk-glm-test", upstream.lastReq.Header.Get("Authorization"))
	require.Equal(t, "glm-5.2", gjson.GetBytes(upstream.lastBody, "model").String())
	require.Equal(t, 6, result.Usage.InputTokens)
	require.Equal(t, 2, result.Usage.OutputTokens)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestFetchUpstreamSupportedModelsGLMOpenAIModeUsesModelsEndpoint(t *testing.T) {
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"data":[{"id":"glm-5.2"},{"id":"glm-5-turbo"}]}`)),
	}}
	svc := NewAccountTestService(nil, nil, nil, nil, nil, upstream, &config.Config{
		Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}},
	}, nil, nil)
	account := &Account{
		ID:          7,
		Platform:    PlatformGLM,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":     "sk-glm-test",
			"base_url":    "https://open.bigmodel.cn/api/paas/v4",
			"compat_mode": "openai",
		},
	}

	models, err := svc.FetchUpstreamSupportedModels(context.Background(), account)

	require.NoError(t, err)
	require.Equal(t, []string{"glm-5-turbo", "glm-5.2"}, models)
	require.NotNil(t, upstream.lastReq)
	require.Equal(t, http.MethodGet, upstream.lastReq.Method)
	require.Equal(t, "https://open.bigmodel.cn/api/paas/v4/models", upstream.lastReq.URL.String())
	require.Equal(t, "Bearer sk-glm-test", upstream.lastReq.Header.Get("Authorization"))
}

func TestFetchUpstreamSupportedModelsGLMAnthropicFallbackBuiltIns(t *testing.T) {
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusNotFound,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"not found"}}`)),
	}}
	svc := NewAccountTestService(nil, nil, nil, nil, nil, upstream, &config.Config{
		Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}},
	}, nil, nil)
	account := &Account{
		ID:          8,
		Platform:    PlatformGLM,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":     "sk-glm-test",
			"base_url":    "https://open.bigmodel.cn/api/anthropic",
			"compat_mode": "anthropic",
		},
	}

	models, err := svc.FetchUpstreamSupportedModels(context.Background(), account)

	require.NoError(t, err)
	require.Contains(t, models, "glm-5.2")
	require.Contains(t, models, "glm-5-turbo")
	require.NotNil(t, upstream.lastReq)
	require.Equal(t, "https://open.bigmodel.cn/api/anthropic/v1/models", upstream.lastReq.URL.String())
	require.Equal(t, "sk-glm-test", upstream.lastReq.Header.Get("x-api-key"))
}

func TestCheckGLMAPIKeyAnthropicModeUsesMessagesProbe(t *testing.T) {
	upstream := &glmProbeHTTPUpstream{responses: []*http.Response{
		{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"code":200,"data":{"limits":[{"type":"TOKENS_LIMIT","unit":3,"number":5,"usage":40000000,"currentValue":10261098,"remaining":29738902,"percentage":25,"nextResetTime":1767373239187}]}}`)),
		},
		{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"content":[{"type":"text","text":"ok"}]}`)),
		},
	}}
	svc := &AccountTestService{httpUpstream: upstream}
	account := &Account{
		ID:          11,
		Platform:    PlatformGLM,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":     "sk-glm-test",
			"compat_mode": "anthropic",
		},
	}

	health, err := svc.CheckAPIKeyValidity(context.Background(), account)

	require.NoError(t, err)
	require.True(t, health.Valid)
	require.False(t, health.Invalid)
	require.NotNil(t, health.ProbeQuota)
	require.Equal(t, "glm_quota_api", health.ProbeQuota.Source)
	require.Len(t, upstream.requests, 2)
	require.Equal(t, http.MethodGet, upstream.requests[0].Method)
	require.Equal(t, "https://open.bigmodel.cn/api/monitor/usage/quota/limit", upstream.requests[0].URL.String())
	require.Equal(t, "sk-glm-test", upstream.requests[0].Header.Get("Authorization"))
	require.Equal(t, http.MethodPost, upstream.requests[1].Method)
	require.Equal(t, "https://open.bigmodel.cn/api/anthropic/v1/messages", upstream.requests[1].URL.String())
	require.Equal(t, "sk-glm-test", upstream.requests[1].Header.Get("x-api-key"))
	require.Equal(t, "glm-5.2", gjson.GetBytes(upstream.bodies[1], "model").String())
}

func TestCheckGLMAPIKeyOpenAIModeUsesModelsEndpoint(t *testing.T) {
	upstream := &glmProbeHTTPUpstream{responses: []*http.Response{
		{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"code":200,"data":{"limits":[{"type":"TOKENS_LIMIT","unit":3,"number":5,"usage":40000000,"currentValue":10261098,"remaining":29738902,"percentage":25,"nextResetTime":1767373239187}]}}`)),
		},
		{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"data":[{"id":"glm-5.2"}]}`)),
		},
	}}
	svc := &AccountTestService{httpUpstream: upstream}
	account := &Account{
		ID:          12,
		Platform:    PlatformGLM,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":     "sk-glm-test",
			"base_url":    "https://open.bigmodel.cn/api/paas/v4",
			"compat_mode": "openai",
		},
	}

	health, err := svc.CheckAPIKeyValidity(context.Background(), account)

	require.NoError(t, err)
	require.True(t, health.Valid)
	require.False(t, health.Invalid)
	require.NotNil(t, health.ProbeQuota)
	require.Equal(t, "glm_quota_api", health.ProbeQuota.Source)
	require.Len(t, upstream.requests, 2)
	require.Equal(t, http.MethodGet, upstream.requests[0].Method)
	require.Equal(t, "https://open.bigmodel.cn/api/monitor/usage/quota/limit", upstream.requests[0].URL.String())
	require.Equal(t, "sk-glm-test", upstream.requests[0].Header.Get("Authorization"))
	require.Equal(t, http.MethodGet, upstream.requests[1].Method)
	require.Equal(t, "https://open.bigmodel.cn/api/paas/v4/models", upstream.requests[1].URL.String())
	require.Equal(t, "Bearer sk-glm-test", upstream.requests[1].Header.Get("Authorization"))
}

func TestGLMOpenAIAccountConnectionDisablesThinking(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := &glmProbeHTTPUpstream{responses: []*http.Response{{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"successful"}}]}`)),
	}}}
	svc := &AccountTestService{httpUpstream: upstream, cfg: &config.Config{
		Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}},
	}}
	account := &Account{
		ID:          13,
		Platform:    PlatformGLM,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":     "sk-glm-test",
			"base_url":    "https://open.bigmodel.cn/api/paas/v4",
			"compat_mode": GLMCompatModeOpenAI,
		},
	}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/test", nil)

	require.NoError(t, svc.testGLMOpenAIAccountConnection(c, account, "glm-5.2", ""))
	require.Len(t, upstream.bodies, 1)
	require.Equal(t, "disabled", gjson.GetBytes(upstream.bodies[0], "thinking.type").String())
	require.Contains(t, recorder.Body.String(), `"type":"test_complete"`)
}

type glmProbeHTTPUpstream struct {
	responses []*http.Response
	requests  []*http.Request
	bodies    [][]byte
}

func (u *glmProbeHTTPUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	u.requests = append(u.requests, req)
	if req != nil && req.Body != nil {
		body, _ := io.ReadAll(req.Body)
		u.bodies = append(u.bodies, body)
		_ = req.Body.Close()
		req.Body = io.NopCloser(bytes.NewReader(body))
	} else {
		u.bodies = append(u.bodies, nil)
	}
	if len(u.responses) == 0 {
		return nil, fmt.Errorf("no mocked response")
	}
	resp := u.responses[0]
	u.responses = u.responses[1:]
	return resp, nil
}

func (u *glmProbeHTTPUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, accountConcurrency)
}
