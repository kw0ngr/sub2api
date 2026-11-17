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
	"github.com/tidwall/gjson"
)

const cyberTestFailure = `{"type":"response.failed","sequence_number":5,"response":{"id":"resp_cyber","status":"failed","error":{"type":"invalid_request","code":"cyber_policy","message":"private upstream refusal text"}}}`

func TestOpenAICyberRedactionSettingHotUpdates(t *testing.T) {
	ctx := context.Background()
	previous, _ := gatewayForwardingCache.Load().(*cachedGatewayForwardingSettings)
	t.Cleanup(func() { gatewayForwardingCache.Store(previous) })
	gatewayForwardingCache.Store(&cachedGatewayForwardingSettings{})
	repo := &cacheControlSettingRepoStub{values: map[string]string{}}
	svc := NewSettingService(repo, &config.Config{})
	require.False(t, svc.IsOpenAICyberSafetyRedactEnabled(ctx))
	require.NoError(t, svc.UpdateSettings(ctx, &SystemSettings{OpenAICyberSafetyRedactEnabled: true}))
	require.Equal(t, "true", repo.values[SettingKeyOpenAICyberSafetyRedact])
	require.True(t, svc.IsOpenAICyberSafetyRedactEnabled(ctx))
	gatewayForwardingCache.Store(&cachedGatewayForwardingSettings{})
	require.True(t, svc.IsOpenAICyberSafetyRedactEnabled(ctx))
	require.NoError(t, svc.UpdateSettings(ctx, &SystemSettings{}))
	require.False(t, svc.IsOpenAICyberSafetyRedactEnabled(ctx))
}

func TestOpenAICyberRedactionExactCodeAndScope(t *testing.T) {
	previous, _ := gatewayForwardingCache.Load().(*cachedGatewayForwardingSettings)
	t.Cleanup(func() { gatewayForwardingCache.Store(previous) })
	svc := &OpenAIGatewayService{settingService: &SettingService{}}
	for _, tt := range []struct {
		name, platform, payload string
		enabled, redact         bool
	}{
		{"disabled", PlatformOpenAI, cyberTestFailure, false, false},
		{"Responses denial", PlatformOpenAI, cyberTestFailure, true, true},
		{"Chat denial", PlatformOpenAI, `{"error":{"code":"cyber_policy","message":"private upstream refusal text"}}`, true, true},
		{"top level error event", PlatformOpenAI, `{"type":"error","code":"cyber_policy","message":"private upstream refusal text"}`, true, true},
		{"same wording", PlatformOpenAI, `{"error":{"code":"other","message":"This content was flagged for possible cybersecurity risk"}}`, true, false},
		{"invalid JSON", PlatformOpenAI, cyberTestFailure + `tail`, true, false},
		{"different provider", PlatformGrok, cyberTestFailure, true, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			gatewayForwardingCache.Store(&cachedGatewayForwardingSettings{
				openAICyberSafetyRedact: tt.enabled,
				expiresAt:               time.Now().Add(time.Minute).UnixNano(),
			})
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

			err := svc.redactOpenAICyberPolicyResponse(c, &Account{Platform: tt.platform}, http.StatusForbidden, []byte(tt.payload))
			if !tt.redact {
				require.NoError(t, err)
				require.False(t, c.Writer.Written())
				require.False(t, c.GetBool(OpsOpenAICyberRedactedKey))
				return
			}
			require.ErrorIs(t, err, errOpenAICyberPolicyRedacted)
			require.Equal(t, http.StatusForbidden, rec.Code)
			require.Equal(t, "error", gjson.Get(rec.Body.String(), "type").String())
			require.Equal(t, OpenAICyberPolicyCode, gjson.Get(rec.Body.String(), "error.code").String())
			require.NotContains(t, rec.Body.String(), "private upstream refusal text")
			require.True(t, c.GetBool(OpsOpenAICyberRedactedKey))
		})
	}
}

func TestOpenAICyberRedactionTerminatesWithoutFailover(t *testing.T) {
	previous, _ := gatewayForwardingCache.Load().(*cachedGatewayForwardingSettings)
	t.Cleanup(func() { gatewayForwardingCache.Store(previous) })
	gatewayForwardingCache.Store(&cachedGatewayForwardingSettings{
		openAICyberSafetyRetry:  true,
		openAICyberSafetyRedact: true,
		expiresAt:               time.Now().Add(time.Minute).UnixNano(),
	})
	ctx := context.Background()
	cfg := &config.Config{}
	repo := &grokProbeStreamAccountRepo{}
	svc := &OpenAIGatewayService{
		cfg: cfg, settingService: &SettingService{},
		rateLimitService: NewRateLimitService(repo, nil, cfg, nil, nil),
	}
	account := &Account{ID: 77, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
	const chatFailure = `{"error":{"code":"cyber_policy","message":"private upstream refusal text"}}`

	t.Run("passthrough HTTP rejection before account cooldown", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
		resp := &http.Response{
			StatusCode: http.StatusForbidden,
			Body:       io.NopCloser(strings.NewReader(chatFailure)),
		}
		err := svc.handleFailoverErrorResponsePassthrough(ctx, resp, c, account, nil)
		require.ErrorIs(t, err, errOpenAICyberPolicyRedacted)
		require.Equal(t, http.StatusForbidden, rec.Code)
		require.Equal(t, OpenAICyberPolicyCode, gjson.Get(rec.Body.String(), "error.code").String())
		require.NotContains(t, rec.Body.String(), "private upstream refusal text")
	})

	for _, tt := range []struct {
		name, path, payload, terminal string
		started                       bool
		forward                       func(*gin.Context, *http.Response) error
	}{
		{"Responses before headers", "/v1/responses", cyberTestFailure, "", false, func(c *gin.Context, resp *http.Response) error {
			_, err := svc.handleStreamingResponse(ctx, resp, c, account, time.Now(), "gpt-test", "gpt-test")
			return err
		}},
		{"Responses after headers", "/v1/responses", cyberTestFailure, "event: response.failed", true, func(c *gin.Context, resp *http.Response) error {
			_, err := svc.handleStreamingResponsePassthrough(ctx, resp, c, account, time.Now(), "gpt-test", "gpt-test")
			return err
		}},
		{"Chat with untyped event", "/v1/chat/completions", chatFailure, "event: error", true, func(c *gin.Context, resp *http.Response) error {
			_, err := svc.handleStreamingResponse(ctx, resp, c, account, time.Now(), "gpt-test", "gpt-test")
			return err
		}},
		{"Messages translation", "/v1/messages", cyberTestFailure, "event: error", true, func(c *gin.Context, resp *http.Response) error {
			_, err := svc.handleAnthropicStreamingResponse(resp, c, account, "gpt-test", "gpt-test", "gpt-test", time.Now())
			return err
		}},
		{"Chat buffered conversion", "/v1/chat/completions", cyberTestFailure, "", false, func(c *gin.Context, resp *http.Response) error {
			_, _, _, err := svc.readOpenAICompatBufferedTerminal(c, account, resp, "test", "")
			return err
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, tt.path, nil)
			if tt.started {
				c.Header("Content-Type", "text/event-stream")
				c.Writer.Flush()
			}
			resp := &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": {"text/event-stream"}},
				Body:       io.NopCloser(strings.NewReader("data: " + tt.payload + "\n\ndata: [DONE]\n\n")),
			}
			err := tt.forward(c, resp)
			require.ErrorIs(t, err, errOpenAICyberPolicyRedacted)
			var failover *UpstreamFailoverError
			require.NotErrorAs(t, err, &failover)
			require.True(t, c.GetBool(OpsOpenAICyberRedactedKey))
			require.Equal(t, http.StatusOK, c.GetInt(OpsUpstreamStatusCodeKey))
			if tt.started {
				require.Equal(t, http.StatusOK, rec.Code)
				require.Contains(t, rec.Body.String(), tt.terminal)
				if tt.path == "/v1/responses" {
					require.Contains(t, rec.Body.String(), `"id":"resp_cyber"`)
					require.Contains(t, rec.Body.String(), `"sequence_number":5`)
				}
			} else {
				require.Equal(t, http.StatusForbidden, rec.Code)
			}
			require.Contains(t, rec.Body.String(), `"code":"cyber_policy"`)
			require.NotContains(t, rec.Body.String(), "private upstream refusal text")
			require.NotContains(t, rec.Body.String(), "[DONE]")
		})
	}
	require.Zero(t, repo.setErrorCalls)
	require.Zero(t, repo.setSchedulableCalls)
	require.Zero(t, repo.rateLimitedCalls)
	require.Zero(t, repo.tempCalls)
	require.Zero(t, repo.modelCooldownCalls)
}
