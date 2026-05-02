//go:build unit

package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
)

// --- shared test helpers ---

type queuedHTTPUpstream struct {
	responses []*http.Response
	requests  []*http.Request
	tlsFlags  []bool
}

func (u *queuedHTTPUpstream) Do(_ *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	return nil, fmt.Errorf("unexpected Do call")
}

func (u *queuedHTTPUpstream) DoWithTLS(req *http.Request, _ string, _ int64, _ int, profile *tlsfingerprint.Profile) (*http.Response, error) {
	u.requests = append(u.requests, req)
	u.tlsFlags = append(u.tlsFlags, profile != nil)
	if len(u.responses) == 0 {
		return nil, fmt.Errorf("no mocked response")
	}
	resp := u.responses[0]
	u.responses = u.responses[1:]
	return resp, nil
}

func newJSONResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

// --- test functions ---

func newTestContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/1/test", nil)
	return c, rec
}

func newSoraTestContext() (*gin.Context, *httptest.ResponseRecorder) {
	return newTestContext()
}

func newOpenAISuccessStream(text string) io.ReadCloser {
	return io.NopCloser(strings.NewReader(
		fmt.Sprintf("data: {\"type\":\"response.output_text.delta\",\"delta\":%q}\n\n", text) +
			"data: {\"type\":\"response.completed\"}\n\n",
	))
}

type openAIAccountTestRepo struct {
	mockAccountRepoForGemini
	updatedExtra  map[string]any
	rateLimitedID int64
	rateLimitedAt *time.Time
}

func (r *openAIAccountTestRepo) UpdateExtra(_ context.Context, _ int64, updates map[string]any) error {
	r.updatedExtra = updates
	return nil
}

func (r *openAIAccountTestRepo) SetRateLimited(_ context.Context, id int64, resetAt time.Time) error {
	r.rateLimitedID = id
	r.rateLimitedAt = &resetAt
	return nil
}

func TestAccountTestService_OpenAISuccessPersistsSnapshotFromHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, recorder := newTestContext()

	resp := newJSONResponse(http.StatusOK, "")
	resp.Body = newOpenAISuccessStream("successful")
	resp.Header.Set("x-codex-primary-used-percent", "88")
	resp.Header.Set("x-codex-primary-reset-after-seconds", "604800")
	resp.Header.Set("x-codex-primary-window-minutes", "10080")
	resp.Header.Set("x-codex-secondary-used-percent", "42")
	resp.Header.Set("x-codex-secondary-reset-after-seconds", "18000")
	resp.Header.Set("x-codex-secondary-window-minutes", "300")

	repo := &openAIAccountTestRepo{}
	upstream := &queuedHTTPUpstream{responses: []*http.Response{resp}}
	svc := &AccountTestService{accountRepo: repo, httpUpstream: upstream}
	account := &Account{
		ID:          89,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{"access_token": "test-token"},
	}

	err := svc.testOpenAIAccountConnection(ctx, account, "gpt-5.4")
	require.NoError(t, err)
	require.NotEmpty(t, repo.updatedExtra)
	require.Equal(t, 42.0, repo.updatedExtra["codex_5h_used_percent"])
	require.Equal(t, 88.0, repo.updatedExtra["codex_7d_used_percent"])
	require.Contains(t, recorder.Body.String(), "test_complete")
}

func TestAccountTestService_OpenAIStreamEOFBeforeCompletedFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, recorder := newTestContext()

	resp := newJSONResponse(http.StatusOK, "")
	resp.Body = io.NopCloser(strings.NewReader(`data: {"type":"response.output_text.delta","delta":"successful"}

`))

	upstream := &queuedHTTPUpstream{responses: []*http.Response{resp}}
	svc := &AccountTestService{httpUpstream: upstream}
	account := &Account{
		ID:          90,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{"access_token": "test-token"},
	}

	err := svc.testOpenAIAccountConnection(ctx, account, "gpt-5.4")
	require.Error(t, err)
	require.Contains(t, recorder.Body.String(), "Stream ended before response.completed")
	require.NotContains(t, recorder.Body.String(), `"success":true`)
}

func TestAccountTestService_OpenAIStreamResponseDoneCompletes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, recorder := newTestContext()

	resp := newJSONResponse(http.StatusOK, "")
	resp.Body = io.NopCloser(strings.NewReader(`data: {"type":"response.output_text.delta","delta":"successful"}

data: {"type":"response.done"}

`))

	upstream := &queuedHTTPUpstream{responses: []*http.Response{resp}}
	svc := &AccountTestService{httpUpstream: upstream}
	account := &Account{
		ID:          91,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{"access_token": "test-token"},
	}

	err := svc.testOpenAIAccountConnection(ctx, account, "gpt-5.4")
	require.NoError(t, err)
	require.Contains(t, recorder.Body.String(), "test_complete")
}

func TestAccountTestService_OpenAI429PersistsSnapshotAndRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := newTestContext()

	resp := newJSONResponse(http.StatusTooManyRequests, `{"error":{"type":"usage_limit_reached","message":"limit reached"}}`)
	resp.Header.Set("x-codex-primary-used-percent", "100")
	resp.Header.Set("x-codex-primary-reset-after-seconds", "604800")
	resp.Header.Set("x-codex-primary-window-minutes", "10080")
	resp.Header.Set("x-codex-secondary-used-percent", "100")
	resp.Header.Set("x-codex-secondary-reset-after-seconds", "18000")
	resp.Header.Set("x-codex-secondary-window-minutes", "300")

	repo := &openAIAccountTestRepo{}
	upstream := &queuedHTTPUpstream{responses: []*http.Response{resp}}
	svc := &AccountTestService{accountRepo: repo, httpUpstream: upstream}
	account := &Account{
		ID:          88,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{"access_token": "test-token"},
	}

	err := svc.testOpenAIAccountConnection(ctx, account, "gpt-5.4")
	require.Error(t, err)
	require.NotEmpty(t, repo.updatedExtra)
	require.Equal(t, 100.0, repo.updatedExtra["codex_5h_used_percent"])
	require.Equal(t, int64(88), repo.rateLimitedID)
	require.NotNil(t, repo.rateLimitedAt)
	require.NotNil(t, account.RateLimitResetAt)
}

func TestAccountTestService_OpenAIApiKeyUsesV1ResponsesEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, recorder := newSoraTestContext()

	resp := newJSONResponse(http.StatusOK, "")
	resp.Body = newOpenAISuccessStream("successful")

	upstream := &queuedHTTPUpstream{responses: []*http.Response{resp}}
	svc := &AccountTestService{
		httpUpstream: upstream,
		cfg:          &config.Config{},
	}
	account := &Account{
		ID:          90,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{"api_key": "sk-test", "base_url": "https://api.openai.com"},
	}

	err := svc.testOpenAIAccountConnection(ctx, account, "gpt-5.4")
	require.NoError(t, err)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, "https://api.openai.com/v1/responses", upstream.requests[0].URL.String())
	require.Contains(t, recorder.Body.String(), "test_complete")
}

func TestAccountTestService_DeepSeekAPIKeyUsesAnthropicCompatibleBaseURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, recorder := newSoraTestContext()

	resp := newJSONResponse(http.StatusOK, "")
	resp.Body = io.NopCloser(strings.NewReader(`data: {"type":"content_block_delta","delta":{"text":"successful"}}

data: {"type":"message_stop"}

`))

	upstream := &queuedHTTPUpstream{responses: []*http.Response{resp}}
	svc := &AccountTestService{
		httpUpstream: upstream,
		cfg:          &config.Config{},
	}
	account := &Account{
		ID:          92,
		Platform:    PlatformDeepSeek,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{"api_key": "sk-test", "base_url": "https://api.deepseek.com"},
	}

	err := svc.testClaudeAccountConnection(ctx, account, "deepseek-v4-flash")
	require.NoError(t, err)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, "https://api.deepseek.com/anthropic/v1/messages?beta=true", upstream.requests[0].URL.String())
	require.Equal(t, "sk-test", getHeaderRaw(upstream.requests[0].Header, "x-api-key"))
	require.Empty(t, getHeaderRaw(upstream.requests[0].Header, "authorization"))
	require.Contains(t, recorder.Body.String(), "test_complete")
}

func TestAccountTestService_OpenRouterAPIKeyUsesAnthropicCompatibleBaseURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, recorder := newSoraTestContext()

	resp := newJSONResponse(http.StatusOK, "")
	resp.Body = io.NopCloser(strings.NewReader(`data: {"type":"content_block_delta","delta":{"text":"successful"}}

data: {"type":"message_stop"}

`))

	upstream := &queuedHTTPUpstream{responses: []*http.Response{resp}}
	svc := &AccountTestService{
		httpUpstream: upstream,
		cfg:          &config.Config{},
	}
	account := &Account{
		ID:          93,
		Platform:    PlatformOpenRouter,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{"api_key": "sk-or-v1-test", "base_url": "https://openrouter.ai/api/v1"},
	}

	err := svc.testClaudeAccountConnection(ctx, account, "anthropic/claude-sonnet-4")
	require.NoError(t, err)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, "https://openrouter.ai/api/v1/messages?beta=true", upstream.requests[0].URL.String())
	require.Equal(t, "Bearer sk-or-v1-test", getHeaderRaw(upstream.requests[0].Header, "authorization"))
	require.Empty(t, getHeaderRaw(upstream.requests[0].Header, "x-api-key"))
	require.Contains(t, recorder.Body.String(), "test_complete")
}

// openAIStreamTextErrorRepo tracks SetError and SetSchedulable calls.
type openAIStreamTextErrorRepo struct {
	mockAccountRepoForGemini
	setErrorCalls       int
	lastErrorMsg        string
	setSchedulableCalls int
	lastSchedulable     bool
}

func (r *openAIStreamTextErrorRepo) SetError(_ context.Context, _ int64, errorMsg string) error {
	r.setErrorCalls++
	r.lastErrorMsg = errorMsg
	return nil
}

func (r *openAIStreamTextErrorRepo) SetSchedulable(_ context.Context, _ int64, schedulable bool) error {
	r.setSchedulableCalls++
	r.lastSchedulable = schedulable
	return nil
}

func TestAccountTestService_OpenAIApiKey_StreamBehavior(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name        string
		sseBody     string
		wantDisable bool // expect SetError+SetSchedulable(false)
		wantSuccess bool // expect test_complete in output
	}{
		{
			// Single delta matching known error phrase, no response.completed → disable
			name: "single_delta_error_phrase_no_completed",
			sseBody: "data: {\"type\":\"response.output_text.delta\",\"delta\":\"Your account is not active, please check your billing details on our website.\"}\n\n" +
				"data: [DONE]\n\n",
			wantDisable: true,
			wantSuccess: false,
		},
		{
			// Completed stream with expected token should not be treated as disable-worthy.
			name: "error_phrase_with_completed_event_not_flagged",
			sseBody: "data: {\"type\":\"response.output_text.delta\",\"delta\":\"Your account is not active, please check your billing details on our website.\"}\n\n" +
				"data: {\"type\":\"response.output_text.delta\",\"delta\":\" successful\"}\n\n" +
				"data: {\"type\":\"response.completed\"}\n\n",
			wantDisable: false,
			wantSuccess: true,
		},
		{
			// Multiple deltas with the expected token should succeed without disable.
			name: "multiple_deltas_not_flagged",
			sseBody: "data: {\"type\":\"response.output_text.delta\",\"delta\":\"Your account is not active\"}\n\n" +
				"data: {\"type\":\"response.output_text.delta\",\"delta\":\", please check your billing details. successful\"}\n\n" +
				"data: {\"type\":\"response.completed\"}\n\n",
			wantDisable: false,
			wantSuccess: true,
		},
		{
			// Normal successful reply: no disable.
			name: "normal_response_not_flagged",
			sseBody: "data: {\"type\":\"response.output_text.delta\",\"delta\":\"successful\"}\n\n" +
				"data: {\"type\":\"response.completed\"}\n\n",
			wantDisable: false,
			wantSuccess: true,
		},
		{
			// The original bug phrase must not trigger disable when the stream still completes successfully.
			name: "hi_response_not_flagged",
			sseBody: "data: {\"type\":\"response.output_text.delta\",\"delta\":\"Hi \\u2014 what can I help with? successful\"}\n\n" +
				"data: {\"type\":\"response.completed\"}\n\n",
			wantDisable: false,
			wantSuccess: true,
		},
		{
			// Single delta, no completed, but expected token present and no known error phrase → fail without disabling.
			name: "single_delta_unknown_text_not_flagged",
			sseBody: "data: {\"type\":\"response.output_text.delta\",\"delta\":\"successful\"}\n\n" +
				"data: [DONE]\n\n",
			wantDisable: false,
			wantSuccess: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, recorder := newSoraTestContext()

			resp := newJSONResponse(http.StatusOK, "")
			resp.Body = io.NopCloser(strings.NewReader(tc.sseBody))

			repo := &openAIStreamTextErrorRepo{}
			upstream := &queuedHTTPUpstream{responses: []*http.Response{resp}}
			svc := &AccountTestService{
				httpUpstream: upstream,
				accountRepo:  repo,
				cfg:          &config.Config{},
			}
			account := &Account{
				ID:          91,
				Platform:    PlatformOpenAI,
				Type:        AccountTypeAPIKey,
				Schedulable: true,
				Concurrency: 1,
				Credentials: map[string]any{"api_key": "sk-test", "base_url": "https://api.openai.com"},
			}

			_ = svc.testOpenAIAccountConnection(ctx, account, "gpt-5.4")

			if tc.wantDisable {
				require.Equal(t, 1, repo.setErrorCalls, "SetError should be called")
				require.Equal(t, 1, repo.setSchedulableCalls, "SetSchedulable should be called")
				require.False(t, repo.lastSchedulable)
				require.NotContains(t, recorder.Body.String(), "test_complete")
			} else {
				require.Equal(t, 0, repo.setErrorCalls, "SetError should NOT be called")
				require.Equal(t, 0, repo.setSchedulableCalls, "SetSchedulable should NOT be called")
				if tc.wantSuccess {
					require.Contains(t, recorder.Body.String(), "test_complete")
				}
			}
		})
	}
}

func TestIsStreamOnlyErrorText(t *testing.T) {
	cases := []struct {
		text          string
		deltaCount    int
		completedSeen bool
		want          bool
	}{
		// Known error phrase, single delta, no completed → detect
		{"Your account is not active, please check your billing details on our website.", 1, false, true},
		{"account is not active", 1, false, true},
		// response.completed seen → never detect
		{"Your account is not active, please check your billing details on our website.", 1, true, false},
		// Multiple deltas → normal reply
		{"Your account is not active", 2, false, false},
		// Text doesn't match any phrase
		{"Hello! How can I help you?", 1, false, false},
		{"Hi — what can I help with?", 1, false, false},
		{"Sure, happy to help!", 1, false, false},
		{"", 1, false, false},
	}

	for _, tc := range cases {
		got := isStreamOnlyErrorText(tc.text, tc.deltaCount, tc.completedSeen)
		if got != tc.want {
			t.Errorf("text=%q deltaCount=%d completedSeen=%v: want %v, got %v",
				tc.text, tc.deltaCount, tc.completedSeen, tc.want, got)
		}
	}
}
