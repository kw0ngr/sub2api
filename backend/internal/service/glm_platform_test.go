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
	require.True(t, SupportedAPIKeyProbePlatform("glm"))
	platform, ok := NormalizeAPIKeyPlatform("glm")
	require.True(t, ok)
	require.Equal(t, "glm", platform)
}

func TestApplyGLMZCodeMimicHeaders(t *testing.T) {
	tests := []struct {
		name        string
		baseURL     string
		strong      bool
		wantApplied bool
		wantUA      string
	}{
		{"official light", "https://open.bigmodel.cn/api/anthropic", false, true, "ZCode/unknown"},
		{"official strong", "https://api.z.ai/api/anthropic", true, true, "ZCode/3.1.2"},
		{"custom relay", "https://relay.example.test", true, false, "custom-client"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tt.baseURL+"/v1/messages?beta=true", nil)
			req.Header.Set("User-Agent", "custom-client")
			req.Header.Set("x-api-key", "sk-glm-test")

			applied := applyGLMZCodeMimicHeaders(req, newGLMAnthropicAccount(0, tt.baseURL), tt.strong)

			require.Equal(t, tt.wantApplied, applied)
			require.Equal(t, "sk-glm-test", req.Header.Get("x-api-key"))
			require.Equal(t, tt.wantUA, req.Header.Get("User-Agent"))
			if !tt.wantApplied {
				require.Empty(t, getHeaderRaw(req.Header, "X-ZCode-Agent"))
				return
			}
			require.Equal(t, "https://zcode.z.ai", getHeaderRaw(req.Header, "HTTP-Referer"))
			require.Equal(t, "Z Code@electron", getHeaderRaw(req.Header, "X-Title"))
			require.Equal(t, "glm", getHeaderRaw(req.Header, "X-ZCode-Agent"))
			if tt.strong {
				require.Equal(t, "3.1.2", getHeaderRaw(req.Header, "X-ZCode-App-Version"))
				require.Equal(t, "darwin-arm64", getHeaderRaw(req.Header, "X-Platform"))
			} else {
				require.Empty(t, getHeaderRaw(req.Header, "X-ZCode-App-Version"))
				require.Empty(t, getHeaderRaw(req.Header, "X-Platform"))
			}
		})
	}
}

func TestGatewayServiceBuildUpstreamRequestGLMZCodeMimic(t *testing.T) {
	tests := []struct {
		name     string
		baseURL  string
		settings map[string]string
		clientUA string
		wantURL  string
		wantUA   string
		wantApp  string
		wantSkip bool
	}{
		{
			name:     "official light",
			baseURL:  "https://open.bigmodel.cn/api/anthropic",
			clientUA: "curl/8",
			wantURL:  "https://open.bigmodel.cn/api/anthropic/v1/messages?beta=true",
			wantUA:   "ZCode/unknown",
		},
		{
			name:     "official strong default base",
			settings: map[string]string{SettingKeyEnableGLMZCodeStrongMimic: "true"},
			wantURL:  "https://open.bigmodel.cn/api/anthropic/v1/messages?beta=true",
			wantUA:   "ZCode/3.1.2",
			wantApp:  "3.1.2",
		},
		{
			name:     "custom relay skips mimic",
			baseURL:  "https://relay.example.test/v1",
			settings: map[string]string{SettingKeyEnableGLMZCodeStrongMimic: "true"},
			clientUA: "custom-client",
			wantURL:  "https://relay.example.test/v1/messages?beta=true",
			wantUA:   "custom-client",
			wantSkip: true,
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := buildGLMUpstreamRequestForTest(t, int64(81000+i), tt.baseURL, tt.settings, tt.clientUA)

			require.Equal(t, tt.wantURL, req.URL.String())
			require.Equal(t, tt.wantUA, req.Header.Get("User-Agent"))
			if tt.wantSkip {
				require.Empty(t, getHeaderRaw(req.Header, "X-ZCode-Agent"))
				return
			}
			require.Equal(t, "glm", getHeaderRaw(req.Header, "X-ZCode-Agent"))
			require.Equal(t, tt.wantApp, getHeaderRaw(req.Header, "X-ZCode-App-Version"))
		})
	}
}

func newGLMAnthropicAccount(id int64, baseURL string) *Account {
	credentials := map[string]any{
		"api_key":     "sk-glm-test",
		"compat_mode": "anthropic",
	}
	if baseURL != "" {
		credentials["base_url"] = baseURL
	}
	return &Account{ID: id, Platform: PlatformGLM, Type: AccountTypeAPIKey, Credentials: credentials}
}

func buildGLMUpstreamRequestForTest(t *testing.T, accountID int64, baseURL string, settings map[string]string, clientUA string) *http.Request {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	if clientUA != "" {
		c.Request.Header.Set("User-Agent", clientUA)
	}
	svc := &GatewayService{
		cfg:            &config.Config{},
		settingService: newGatewayForwardingSettingService(t, settings),
	}
	req, err := svc.buildUpstreamRequest(
		context.Background(),
		c,
		newGLMAnthropicAccount(accountID, baseURL),
		[]byte(`{"model":"glm-5.2","messages":[{"role":"user","content":"hi"}]}`),
		"sk-glm-test",
		"apikey",
		"glm-5.2",
		false,
		false,
	)
	require.NoError(t, err)
	return req
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
	svc := NewAccountTestService(nil, nil, nil, nil, upstream, &config.Config{
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
	svc := NewAccountTestService(nil, nil, nil, nil, upstream, &config.Config{
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
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"content":[{"type":"text","text":"ok"}]}`)),
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
	require.NotNil(t, upstream.lastReq)
	require.Equal(t, http.MethodPost, upstream.lastReq.Method)
	require.Equal(t, "https://open.bigmodel.cn/api/anthropic/v1/messages", upstream.lastReq.URL.String())
	require.Equal(t, "sk-glm-test", upstream.lastReq.Header.Get("x-api-key"))
	require.Equal(t, "glm-5.2", gjson.GetBytes(upstream.lastBody, "model").String())
}

func TestCheckGLMAPIKeyOpenAIModeUsesModelsEndpoint(t *testing.T) {
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"data":[{"id":"glm-5.2"}]}`)),
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
	require.NotNil(t, upstream.lastReq)
	require.Equal(t, http.MethodGet, upstream.lastReq.Method)
	require.Equal(t, "https://open.bigmodel.cn/api/paas/v4/models", upstream.lastReq.URL.String())
	require.Equal(t, "Bearer sk-glm-test", upstream.lastReq.Header.Get("Authorization"))
}
