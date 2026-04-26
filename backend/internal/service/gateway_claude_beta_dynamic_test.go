package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestBuildAuth2APIDynamicBetaTokens_NonHaiku(t *testing.T) {
	got := buildAuth2APIDynamicBetaTokens("claude-sonnet-4-6", false)
	require.Equal(t, []string{
		claude.BetaClaudeCode,
		claude.BetaOAuth,
		claude.BetaInterleavedThinking,
		betaRedactThinking,
		betaContextManagement,
		betaPromptCachingScope,
		betaAdvancedToolUse,
		betaEffort,
	}, got)
}

func TestBuildAuth2APIDynamicBetaTokens_HaikuStructured(t *testing.T) {
	got := buildAuth2APIDynamicBetaTokens("claude-haiku-4-5-20251001", true)
	require.Equal(t, []string{
		claude.BetaOAuth,
		claude.BetaInterleavedThinking,
		betaRedactThinking,
		betaContextManagement,
		betaPromptCachingScope,
		betaStructuredOutputs,
	}, got)
}

func TestGatewayService_GetBetaHeader_ClientProvided(t *testing.T) {
	svc := &GatewayService{}

	withoutOAuth := svc.getBetaHeader("claude-sonnet-4-6", []byte(`{}`), "interleaved-thinking-2025-05-14,foo-beta")
	require.Equal(t, "oauth-2025-04-20,interleaved-thinking-2025-05-14,foo-beta", withoutOAuth)

	withOAuth := svc.getBetaHeader("claude-sonnet-4-6", []byte(`{}`), "oauth-2025-04-20,foo-beta")
	require.Equal(t, "oauth-2025-04-20,foo-beta", withOAuth)
}

func TestGatewayService_GetBetaHeader_DynamicStructured(t *testing.T) {
	svc := &GatewayService{}
	body := []byte(`{"output_config":{"type":"json_schema","json_schema":{"name":"x","schema":{"type":"object"}}}}`)
	got := svc.getBetaHeader("claude-sonnet-4-6", body, "")
	require.Equal(t, strings.Join([]string{
		claude.BetaClaudeCode,
		claude.BetaOAuth,
		claude.BetaInterleavedThinking,
		betaRedactThinking,
		betaContextManagement,
		betaPromptCachingScope,
		betaAdvancedToolUse,
		betaEffort,
		betaStructuredOutputs,
	}, ","), got)
}

func TestGatewayService_BuildUpstreamRequest_OAuthMimicUsesDynamicBetas(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	svc := &GatewayService{}
	account := &Account{
		ID:          1001,
		Name:        "oauth-dynamic-beta",
		Platform:    PlatformAnthropic,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
	}

	req, err := svc.buildUpstreamRequest(
		context.Background(),
		c,
		account,
		[]byte(`{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":[{"type":"text","text":"hi"}]}]}`),
		"oauth-token",
		"oauth",
		"claude-sonnet-4-6",
		false,
		true,
	)
	require.NoError(t, err)
	beta := getHeaderRaw(req.Header, "anthropic-beta")
	require.Contains(t, beta, claude.BetaClaudeCode)
	require.Contains(t, beta, claude.BetaOAuth)
	require.Contains(t, beta, claude.BetaRedactThinking)
	require.Contains(t, beta, claude.BetaPromptCachingScope)
	require.Contains(t, beta, claude.BetaExtendedCacheTTL)
	require.Contains(t, beta, claude.BetaEffort)
	require.NotContains(t, beta, betaAdvancedToolUse)
	require.Equal(t, claude.DefaultHeaders["User-Agent"], getHeaderRaw(req.Header, "User-Agent"))
	require.Equal(t, claude.DefaultHeaders["X-Stainless-OS"], getHeaderRaw(req.Header, "X-Stainless-OS"))
	require.Equal(t, "application/json", getHeaderRaw(req.Header, "Accept"))
}

func TestGatewayService_BuildUpstreamRequest_OAuthMimicSyncsBillingVersionToForcedUA(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	cache := &recordingIdentityCache{fp: &Fingerprint{
		ClientID:                "cached-client-id",
		UserAgent:               "claude-cli/2.1.88 (stale, cli)",
		StainlessLang:           "js",
		StainlessPackageVersion: "0.74.0",
		StainlessOS:             "Linux",
		StainlessArch:           "arm64",
		StainlessRuntime:        "node",
		StainlessRuntimeVersion: "v22.13.0",
		UpdatedAt:               time.Now().Unix(),
	}}
	svc := &GatewayService{identityService: NewIdentityService(cache)}
	account := &Account{
		ID:          1003,
		Name:        "oauth-stale-fingerprint",
		Platform:    PlatformAnthropic,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
	}

	req, err := svc.buildUpstreamRequest(
		context.Background(),
		c,
		account,
		[]byte(`{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":[{"type":"text","text":"01234567890123456789012345"}]}]}`),
		"oauth-token",
		"oauth",
		"claude-sonnet-4-6",
		false,
		true,
	)
	require.NoError(t, err)
	rawBody, err := io.ReadAll(req.Body)
	require.NoError(t, err)

	require.Equal(t, claude.DefaultHeaders["User-Agent"], getHeaderRaw(req.Header, "User-Agent"))
	require.Contains(t, gjson.GetBytes(rawBody, "system.0.text").String(), "cc_version="+claude.CLICurrentVersion)
	require.NotContains(t, gjson.GetBytes(rawBody, "system.0.text").String(), "cc_version=2.1.88")
}

func TestGatewayService_BuildCountTokensRequest_OAuthMimicUsesTokenCountingBeta(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages/count_tokens", nil)

	svc := &GatewayService{}
	account := &Account{
		ID:          1002,
		Name:        "oauth-count-token-beta",
		Platform:    PlatformAnthropic,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
	}

	req, err := svc.buildCountTokensRequest(
		context.Background(),
		c,
		account,
		[]byte(`{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":[{"type":"text","text":"hi"}]}]}`),
		"oauth-token",
		"oauth",
		"claude-sonnet-4-6",
		true,
	)
	require.NoError(t, err)
	beta := getHeaderRaw(req.Header, "anthropic-beta")
	require.Contains(t, beta, claude.BetaClaudeCode)
	require.Contains(t, beta, claude.BetaOAuth)
	require.Contains(t, beta, claude.BetaExtendedCacheTTL)
	require.Contains(t, beta, claude.BetaTokenCounting)
}
