package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func upstreamModelSyncTestConfig() *config.Config {
	return &config.Config{
		Security: config.SecurityConfig{
			URLAllowlist: config.URLAllowlistConfig{Enabled: false},
		},
	}
}

func TestBuildV1ModelsURL(t *testing.T) {
	t.Parallel()

	require.Equal(t, "https://api.anthropic.com/v1/models", buildV1ModelsURL("https://api.anthropic.com"))
	require.Equal(t, "https://api.anthropic.com/v1/models", buildV1ModelsURL("https://api.anthropic.com/v1"))
	require.Equal(t, "https://api.anthropic.com/v1/models", buildV1ModelsURL("https://api.anthropic.com/v1/models"))
	require.Equal(t, "https://gateway.example.com/antigravity/v1/models", buildV1ModelsURL("https://gateway.example.com/antigravity/"))
}

func TestBuildGeminiModelsURL(t *testing.T) {
	t.Parallel()

	require.Equal(t, "https://generativelanguage.googleapis.com/v1beta/models", buildGeminiModelsURL("https://generativelanguage.googleapis.com"))
	require.Equal(t, "https://generativelanguage.googleapis.com/v1beta/models", buildGeminiModelsURL("https://generativelanguage.googleapis.com/v1beta"))
	require.Equal(t, "https://generativelanguage.googleapis.com/v1beta/models", buildGeminiModelsURL("https://generativelanguage.googleapis.com/v1beta/models"))
}

func TestExtractUpstreamModelIDs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want []string
	}{
		{
			name: "openai and anthropic data array",
			body: `{"data":[{"id":"claude-sonnet-4-5"},{"id":"gpt-5"},{"id":"gpt-5"},{"id":""}]}`,
			want: []string{"claude-sonnet-4-5", "gpt-5"},
		},
		{
			name: "gemini models array strips prefix",
			body: `{"models":[{"name":"models/gemini-2.5-pro"},{"name":"gemini-2.5-flash"}]}`,
			want: []string{"gemini-2.5-flash", "gemini-2.5-pro"},
		},
		{
			name: "top level array",
			body: `[{"id":"z-model"},{"name":"models/a-model"}]`,
			want: []string{"a-model", "z-model"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := extractUpstreamModelIDs([]byte(tt.body))
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestBuildUpstreamModelsRequestsForAPIKeyAccounts(t *testing.T) {
	t.Parallel()

	svc := &AccountTestService{cfg: upstreamModelSyncTestConfig()}
	ctx := context.Background()

	anthropicReq, err := svc.buildAnthropicUpstreamModelsRequest(ctx, &Account{
		Platform: PlatformAnthropic,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "anthropic-key",
			"base_url": "https://anthropic.example.com/v1",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "https://anthropic.example.com/v1/models", anthropicReq.URL.String())
	require.Equal(t, "anthropic-key", anthropicReq.Header.Get("x-api-key"))
	require.Equal(t, "2023-06-01", anthropicReq.Header.Get("anthropic-version"))

	openAIReq, err := svc.buildOpenAIUpstreamModelsRequest(ctx, &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "openai-key",
			"base_url": "https://openai.example.com",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "https://openai.example.com/v1/models", openAIReq.URL.String())
	require.Equal(t, "Bearer openai-key", openAIReq.Header.Get("Authorization"))

	grokOAuthReq, err := svc.buildOpenAIUpstreamModelsRequest(ctx, &Account{
		Platform: PlatformGrok,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token": "xai-token",
			"base_url":     "https://cli-chat-proxy.grok.com/v1",
			"headers": map[string]any{
				"x-grok-client-version": "9.9.9-test",
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "https://cli-chat-proxy.grok.com/v1/models", grokOAuthReq.URL.String())
	require.Equal(t, "Bearer xai-token", grokOAuthReq.Header.Get("Authorization"))
	require.Equal(t, "9.9.9-test", grokOAuthReq.Header.Get("x-grok-client-version"))
	require.Equal(t, "xai-grok-cli", grokOAuthReq.Header.Get("x-xai-token-auth"))
	require.Equal(t, "interactive", grokOAuthReq.Header.Get("x-grok-client-mode"))
	require.NotEmpty(t, grokOAuthReq.Header.Get("User-Agent"))

	geminiReq, err := svc.buildGeminiUpstreamModelsRequest(ctx, &Account{
		Platform: PlatformGemini,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "gemini-key",
			"base_url": "https://generativelanguage.googleapis.com/v1beta",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "https://generativelanguage.googleapis.com/v1beta/models", geminiReq.URL.String())
	require.Equal(t, "gemini-key", geminiReq.Header.Get("x-goog-api-key"))

	antigravityReq, err := svc.buildAntigravityAPIKeyModelsRequest(ctx, &Account{
		Platform: PlatformAntigravity,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "antigravity-key",
			"base_url": "https://gateway.example.com/antigravity",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "https://gateway.example.com/antigravity/v1/models", antigravityReq.URL.String())
	require.Equal(t, "antigravity-key", antigravityReq.Header.Get("x-api-key"))
}

func TestBuildAntigravityAPIKeyModelsRequestRejectsOfficialCloudCodeBase(t *testing.T) {
	t.Parallel()

	svc := &AccountTestService{cfg: upstreamModelSyncTestConfig()}
	_, err := svc.buildAntigravityAPIKeyModelsRequest(context.Background(), &Account{
		Platform: PlatformAntigravity,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "antigravity-key",
			"base_url": "https://cloudcode-pa.googleapis.com",
		},
	})
	require.Error(t, err)

	var syncErr *UpstreamModelSyncError
	require.True(t, errors.As(err, &syncErr))
	require.Equal(t, UpstreamModelSyncErrorUnsupported, syncErr.Kind)
	require.Contains(t, syncErr.SafeMessage(), "compatible gateway")
}

func TestBuildAnthropicUpstreamModelsRequestRejectsBedrock(t *testing.T) {
	t.Parallel()

	svc := &AccountTestService{cfg: upstreamModelSyncTestConfig()}
	_, err := svc.buildAnthropicUpstreamModelsRequest(context.Background(), &Account{
		Platform: PlatformAnthropic,
		Type:     AccountTypeBedrock,
	})
	require.Error(t, err)

	var syncErr *UpstreamModelSyncError
	require.True(t, errors.As(err, &syncErr))
	require.Equal(t, UpstreamModelSyncErrorUnsupported, syncErr.Kind)
}

func TestFetchUpstreamSupportedModelsParsesOpenAIResponse(t *testing.T) {
	t.Parallel()

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"data":[{"id":"gpt-5"},{"id":"gpt-5"},{"name":"o3"}]}`)),
	}}
	svc := &AccountTestService{
		httpUpstream: upstream,
		cfg:          upstreamModelSyncTestConfig(),
	}

	models, err := svc.FetchUpstreamSupportedModels(context.Background(), &Account{
		ID:       7,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "openai-key",
			"base_url": "https://openai.example.com/v1",
		},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"gpt-5", "o3"}, models)
	require.Equal(t, "https://openai.example.com/v1/models", upstream.lastReq.URL.String())
	require.Equal(t, "Bearer openai-key", upstream.lastReq.Header.Get("Authorization"))
}

func TestFetchUpstreamSupportedModelsDoesNotExposeUpstreamBody(t *testing.T) {
	t.Parallel()

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusBadGateway,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":"SECRET_TOKEN should not be exposed"}`)),
	}}
	svc := &AccountTestService{
		httpUpstream: upstream,
		cfg:          upstreamModelSyncTestConfig(),
	}

	_, err := svc.FetchUpstreamSupportedModels(context.Background(), &Account{
		ID:       8,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "openai-key",
			"base_url": "https://openai.example.com/v1",
		},
	})
	require.Error(t, err)
	require.NotContains(t, err.Error(), "SECRET_TOKEN")

	var syncErr *UpstreamModelSyncError
	require.True(t, errors.As(err, &syncErr))
	require.Equal(t, UpstreamModelSyncErrorUpstream, syncErr.Kind)
	require.NotContains(t, syncErr.SafeMessage(), "SECRET_TOKEN")
	require.Contains(t, syncErr.SafeMessage(), "HTTP 502")
}

type task6ModelMetadataRepoStub struct {
	AccountRepository
	accountID int64
	updates   map[string]any
	calls     int
}

func (r *task6ModelMetadataRepoStub) UpdateExtra(_ context.Context, id int64, updates map[string]any) error {
	r.accountID = id
	r.updates = updates
	r.calls++
	return nil
}

func TestUpstreamModelMetadataPersistsCompleteSubsetSourceProvenanceAndOfficialDefaults(t *testing.T) {
	// Given
	repo := &task6ModelMetadataRepoStub{}
	upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"data":[
		{"id":"gpt-6-astra","display_name":"Live Astra","reasoning":true,"default_reasoning_level":"high","supported_reasoning_levels":["max","low","high","low","invalid"],"input_modalities":["image","text","audio"],"context_window":1050000,"codex_tool_capabilities":{"supports_search_tool":true,"apply_patch_tool_type":"freeform","comp_hash":"3000","use_responses_lite":false,"tool_mode":null}},
		{"id":"gpt-5.6-sol"},
		{"id":"custom-incomplete","reasoning":true}
	]}`))}}
	svc := &AccountTestService{accountRepo: repo, httpUpstream: upstream, cfg: upstreamModelSyncTestConfig()}
	account := &Account{ID: 701, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "openai-key", "base_url": "https://api.openai.example/v1"}}

	// When
	models, err := svc.FetchUpstreamSupportedModels(context.Background(), account)

	// Then
	require.NoError(t, err)
	require.Equal(t, []string{"custom-incomplete", "gpt-5.6-sol", "gpt-6-astra"}, models)
	require.Equal(t, int64(701), repo.accountID)
	snapshot := task6SnapshotFromUpdate(t, repo.updates)
	require.Equal(t, "mixed", snapshot["source"])
	require.NotEmpty(t, snapshot["synced_at"])
	syncedAt, ok := snapshot["synced_at"].(string)
	require.True(t, ok)
	_, err = time.Parse(time.RFC3339, syncedAt)
	require.NoError(t, err)
	modelMap, ok := snapshot["models"].(map[string]any)
	require.True(t, ok)
	require.NotContains(t, modelMap, "custom-incomplete")
	astra, ok := modelMap["gpt-6-astra"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "Live Astra", astra["display_name"])
	require.Equal(t, []any{"local_official_default", "upstream"}, astra["sources"])
	require.Equal(t, true, astra["reasoning"])
	require.Equal(t, "high", astra["default_reasoning_level"])
	require.Equal(t, []any{"low", "high", "max"}, astra["supported_reasoning_levels"])
	require.Equal(t, []any{"image", "text"}, astra["input_modalities"])
	require.Equal(t, float64(128000), astra["max_output_tokens"])
	sol, ok := modelMap["gpt-5.6-sol"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, []any{"none", "low", "medium", "high", "xhigh", "max"}, sol["supported_reasoning_levels"])
	require.Equal(t, float64(128000), sol["max_output_tokens"])
}

func TestUpstreamModelMetadataPartialRefreshUnionsPreviousCompleteEntries(t *testing.T) {
	// Given
	repo := &task6ModelMetadataRepoStub{}
	account := &Account{ID: 702, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "openai-key", "base_url": "https://api.openai.example/v1"}, Extra: map[string]any{
		"upstream_model_metadata": map[string]any{"source": "mixed", "synced_at": "2026-09-06T00:00:00Z", "models": map[string]any{
			"gpt-6-astra":  map[string]any{"id": "gpt-6-astra", "sources": []any{"upstream"}, "reasoning": true, "supported_reasoning_levels": []any{"low", "medium"}, "input_modalities": []any{"text"}, "context_window": float64(1050000), "max_output_tokens": float64(128000), "codex_tool_capabilities": map[string]any{"supports_search_tool": false}},
			"still-listed": map[string]any{"id": "still-listed", "sources": []any{"upstream"}, "reasoning": false, "input_modalities": []any{"text"}, "context_window": float64(64000)},
		}},
	}}
	upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"data":[{"id":"gpt-5.6-terra"}]}`))}}
	svc := &AccountTestService{accountRepo: repo, httpUpstream: upstream, cfg: upstreamModelSyncTestConfig()}

	// When
	_, err := svc.FetchUpstreamSupportedModels(context.Background(), account)

	// Then
	require.NoError(t, err)
	snapshot := task6SnapshotFromUpdate(t, repo.updates)
	modelMap, ok := snapshot["models"].(map[string]any)
	require.True(t, ok)
	require.Contains(t, modelMap, "gpt-6-astra")
	require.Contains(t, modelMap, "gpt-5.6-terra")
	require.Contains(t, modelMap, "still-listed")
	astra, ok := modelMap["gpt-6-astra"].(map[string]any)
	require.True(t, ok)
	capabilities, ok := astra["codex_tool_capabilities"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, false, capabilities["supports_search_tool"])
}

func TestUpstreamModelMetadataDoesNotUpdateOnMalformedEmptyOrNoComplete(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		body    string
		wantErr bool
	}{
		{name: "malformed json", status: http.StatusOK, body: `{`, wantErr: true},
		{name: "empty list", status: http.StatusOK, body: `{"data":[]}`, wantErr: true},
		{name: "no complete custom metadata", status: http.StatusOK, body: `{"data":[{"id":"custom-model","reasoning":true}]}`},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			repo := &task6ModelMetadataRepoStub{}
			upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: tt.status, Body: io.NopCloser(strings.NewReader(tt.body))}}
			svc := &AccountTestService{accountRepo: repo, httpUpstream: upstream, cfg: upstreamModelSyncTestConfig()}
			account := &Account{ID: 703, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, ErrorMessage: "unchanged", Credentials: map[string]any{"api_key": "openai-key", "base_url": "https://api.openai.example/v1"}}

			// When
			_, err := svc.FetchUpstreamSupportedModels(context.Background(), account)

			// Then
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Zero(t, repo.calls)
			require.Equal(t, StatusActive, account.Status)
			require.Equal(t, "unchanged", account.ErrorMessage)
		})
	}
}

func TestUpstreamModelMetadataMediaIncompleteDoesNotBlockAstraPersistence(t *testing.T) {
	// Given
	repo := &task6ModelMetadataRepoStub{}
	upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"data":[{"id":"gpt-6-astra"},{"id":"gpt-image-2","reasoning":false,"input_modalities":["image"]}]}`))}}
	svc := &AccountTestService{accountRepo: repo, httpUpstream: upstream, cfg: upstreamModelSyncTestConfig()}
	account := &Account{ID: 704, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "openai-key", "base_url": "https://api.openai.example/v1"}}

	// When
	_, err := svc.FetchUpstreamSupportedModels(context.Background(), account)

	// Then
	require.NoError(t, err)
	snapshot := task6SnapshotFromUpdate(t, repo.updates)
	modelMap, ok := snapshot["models"].(map[string]any)
	require.True(t, ok)
	require.Contains(t, modelMap, "gpt-6-astra")
	require.NotContains(t, modelMap, "gpt-image-2")
}

func TestUpstreamModelMetadataRollbackGenericAccountDecodeIgnoresSnapshot(t *testing.T) {
	// Given
	raw := []byte(`{"id":705,"name":"rollback","extra":{"upstream_model_metadata":{"source":"mixed","models":{"gpt-6-astra":{"id":"gpt-6-astra"}}},"unknown_future_key":{"nested":true}}}`)

	// When
	var account Account
	err := json.Unmarshal(raw, &account)

	// Then
	require.NoError(t, err)
	require.Contains(t, account.Extra, "upstream_model_metadata")
	require.Contains(t, account.Extra, "unknown_future_key")
}

func task6SnapshotFromUpdate(t *testing.T, updates map[string]any) map[string]any {
	t.Helper()
	require.Contains(t, updates, "upstream_model_metadata")
	body, err := json.Marshal(updates["upstream_model_metadata"])
	require.NoError(t, err)
	var snapshot map[string]any
	require.NoError(t, json.Unmarshal(body, &snapshot))
	return snapshot
}
