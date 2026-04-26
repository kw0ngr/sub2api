package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type gatewayForwardingSettingRepoStub struct {
	values map[string]string
}

func (s *gatewayForwardingSettingRepoStub) Get(context.Context, string) (*Setting, error) {
	return nil, ErrSettingNotFound
}

func (s *gatewayForwardingSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	if v, ok := s.values[key]; ok {
		return v, nil
	}
	return "", ErrSettingNotFound
}

func (s *gatewayForwardingSettingRepoStub) Set(context.Context, string, string) error {
	panic("unexpected Set call")
}

func (s *gatewayForwardingSettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if v, ok := s.values[key]; ok {
			out[key] = v
		}
	}
	return out, nil
}

func (s *gatewayForwardingSettingRepoStub) SetMultiple(context.Context, map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (s *gatewayForwardingSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	out := make(map[string]string, len(s.values))
	for key, value := range s.values {
		out[key] = value
	}
	return out, nil
}

func (s *gatewayForwardingSettingRepoStub) Delete(context.Context, string) error {
	panic("unexpected Delete call")
}

type recordingIdentityCache struct {
	fp       *Fingerprint
	gets     atomic.Int64
	sets     atomic.Int64
	lastSet  *Fingerprint
	maskedID string
}

func (c *recordingIdentityCache) GetFingerprint(context.Context, int64) (*Fingerprint, error) {
	c.gets.Add(1)
	if c.fp == nil {
		return nil, nil
	}
	cp := *c.fp
	return &cp, nil
}

func (c *recordingIdentityCache) SetFingerprint(_ context.Context, _ int64, fp *Fingerprint) error {
	c.sets.Add(1)
	if fp != nil {
		cp := *fp
		c.lastSet = &cp
	}
	return nil
}

func (c *recordingIdentityCache) GetMaskedSessionID(context.Context, int64) (string, error) {
	return c.maskedID, nil
}

func (c *recordingIdentityCache) SetMaskedSessionID(_ context.Context, _ int64, sessionID string) error {
	c.maskedID = sessionID
	return nil
}

func newGatewayForwardingSettingService(t *testing.T, values map[string]string) *SettingService {
	t.Helper()
	gatewayForwardingCache.Store(&cachedGatewayForwardingSettings{expiresAt: time.Now().Add(-time.Second).UnixNano()})
	t.Cleanup(func() {
		gatewayForwardingCache.Store(&cachedGatewayForwardingSettings{expiresAt: time.Now().Add(-time.Second).UnixNano()})
	})
	return NewSettingService(&gatewayForwardingSettingRepoStub{values: values}, &config.Config{})
}

func TestComputeClaudeCodeBillingFingerprint(t *testing.T) {
	message := "01234567890123456789012345"
	got := computeClaudeCodeBillingFingerprint(message, "2.1.92")
	require.Equal(t, "f34", got)
}

func TestGenerateClaudeCodeBillingHeader(t *testing.T) {
	body := []byte(`{"messages":[{"role":"user","content":[{"type":"text","text":"01234567890123456789012345"}]}]}`)
	got := generateClaudeCodeBillingHeader(body, "2.1.92", "cli", "")
	require.Equal(t, "x-anthropic-billing-header: cc_version=2.1.92.f34; cc_entrypoint=cli; cch=00000;", got)
}

func TestEnsureClaudeOAuthSystemCloaking_InsertsBillingAndPrefix(t *testing.T) {
	body := []byte(`{"model":"claude-sonnet-4-6","system":[{"type":"text","text":"custom system"}],"messages":[{"role":"user","content":[{"type":"text","text":"01234567890123456789012345"}]}]}`)
	next, changed := ensureClaudeOAuthSystemCloaking(body, "2.1.92", "cli")
	require.True(t, changed)

	system := gjson.GetBytes(next, "system")
	require.True(t, system.Exists())
	require.True(t, system.IsArray())
	require.GreaterOrEqual(t, len(system.Array()), 3)

	first := system.Array()[0].Get("text").String()
	require.Contains(t, first, "x-anthropic-billing-header:")
	require.Contains(t, first, "cc_version=2.1.92.f34")
	require.Contains(t, first, "cc_entrypoint=cli")

	second := system.Array()[1].Get("text").String()
	require.Equal(t, claudeCodeSystemPrompt, strings.TrimSpace(second))
	require.Equal(t, "custom system", system.Array()[2].Get("text").String())
}

func TestEnsureClaudeOAuthSystemCloaking_PreservesExistingBillingBlock(t *testing.T) {
	existingBilling := "x-anthropic-billing-header: cc_version=2.0.0.abc; cc_entrypoint=cli;"
	body := []byte(`{"model":"claude-sonnet-4-6","system":[{"type":"text","text":"` + existingBilling + `"},{"type":"text","text":"You are Claude Code, Anthropic's official CLI for Claude."},{"type":"text","text":"custom"}],"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]}`)
	next, changed := ensureClaudeOAuthSystemCloaking(body, "2.1.92", "cli")
	require.True(t, changed)

	system := gjson.GetBytes(next, "system")
	require.True(t, system.IsArray())
	items := system.Array()
	require.GreaterOrEqual(t, len(items), 3)
	require.Equal(t, existingBilling, items[0].Get("text").String())
	require.Equal(t, "You are Claude Code, Anthropic's official CLI for Claude.", items[1].Get("text").String())
	require.Equal(t, "custom", items[2].Get("text").String())
	require.Equal(t, 1, strings.Count(system.Raw, "x-anthropic-billing-header"))
}

func TestBuildUpstreamRequest_OAuth_ForcesJSONMetadataAndSessionHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	req.Header.Set("x-api-key", "test-key-session-metadata")
	c.Request = req

	svc := &GatewayService{}
	account := &Account{
		ID:       4242,
		Platform: PlatformAnthropic,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			"account_uuid": "acc-uuid-42",
		},
	}

	body := []byte(`{"model":"claude-sonnet-4-6","metadata":{"user_id":"legacy-user-id"},"messages":[{"role":"user","content":[{"type":"text","text":"01234567890123456789012345"}]}]}`)
	upstreamReq, err := svc.buildUpstreamRequest(context.Background(), c, account, body, "oauth-token", "oauth", "claude-sonnet-4-6", false, false)
	require.NoError(t, err)

	rawBody, err := io.ReadAll(upstreamReq.Body)
	require.NoError(t, err)
	uidRaw := gjson.GetBytes(rawBody, "metadata.user_id").String()
	require.NotEmpty(t, uidRaw)

	parsed := ParseMetadataUserID(uidRaw)
	require.NotNil(t, parsed)
	require.True(t, parsed.IsNewFormat)
	require.NotEmpty(t, parsed.DeviceID)
	require.Equal(t, "acc-uuid-42", parsed.AccountUUID)
	require.NotEmpty(t, parsed.SessionID)

	sessionHeader := getHeaderRaw(upstreamReq.Header, "X-Claude-Code-Session-Id")
	require.NotEmpty(t, sessionHeader)
	require.Equal(t, parsed.SessionID, sessionHeader)
}

func TestBuildUpstreamRequest_OAuth_PreservesMetadataWhenPassthroughEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	req.Header.Set("x-api-key", "test-key-metadata-passthrough")
	c.Request = req

	svc := &GatewayService{
		settingService: newGatewayForwardingSettingService(t, map[string]string{
			SettingKeyEnableFingerprintUnification: "true",
			SettingKeyEnableMetadataPassthrough:    "true",
		}),
	}
	account := &Account{
		ID:       4243,
		Platform: PlatformAnthropic,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			"account_uuid": "acc-uuid-43",
		},
	}

	body := []byte(`{"model":"claude-sonnet-4-6","metadata":{"user_id":"client-user-id"},"messages":[{"role":"user","content":[{"type":"text","text":"01234567890123456789012345"}]}]}`)
	upstreamReq, err := svc.buildUpstreamRequest(context.Background(), c, account, body, "oauth-token", "oauth", "claude-sonnet-4-6", false, false)
	require.NoError(t, err)

	rawBody, err := io.ReadAll(upstreamReq.Body)
	require.NoError(t, err)
	require.Equal(t, "client-user-id", gjson.GetBytes(rawBody, "metadata.user_id").String())
}

func TestBuildUpstreamRequest_OAuth_FingerprintUnificationDisabledPreservesClientHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	req.Header.Set("x-api-key", "test-key-fp-disabled")
	req.Header.Set("User-Agent", "claude-cli/2.1.1 (client, cli)")
	req.Header.Set("X-Stainless-Lang", "client-js")
	req.Header.Set("X-Stainless-Package-Version", "0.11.0")
	c.Request = req

	cache := &recordingIdentityCache{fp: &Fingerprint{
		ClientID:                "cached-client-id",
		UserAgent:               "claude-cli/9.9.9 (cached, cli)",
		StainlessLang:           "cached-js",
		StainlessPackageVersion: "9.9.9",
		StainlessOS:             "cached-os",
		StainlessArch:           "cached-arch",
		StainlessRuntime:        "cached-runtime",
		StainlessRuntimeVersion: "cached-runtime-version",
		UpdatedAt:               time.Now().Unix(),
	}}
	svc := &GatewayService{
		identityService: NewIdentityService(cache),
		settingService: newGatewayForwardingSettingService(t, map[string]string{
			SettingKeyEnableFingerprintUnification: "false",
			SettingKeyEnableMetadataPassthrough:    "true",
		}),
	}
	account := &Account{
		ID:       4244,
		Platform: PlatformAnthropic,
		Type:     AccountTypeOAuth,
	}
	body := []byte(`{"model":"claude-sonnet-4-6","metadata":{"user_id":"client-user-id"},"messages":[{"role":"user","content":[{"type":"text","text":"01234567890123456789012345"}]}]}`)

	upstreamReq, err := svc.buildUpstreamRequest(context.Background(), c, account, body, "oauth-token", "oauth", "claude-sonnet-4-6", false, false)
	require.NoError(t, err)

	require.Equal(t, int64(0), cache.gets.Load(), "fingerprint cache should not be read when unification is disabled")
	require.Equal(t, "claude-cli/2.1.1 (client, cli)", getHeaderRaw(upstreamReq.Header, "User-Agent"))
	require.Equal(t, "client-js", getHeaderRaw(upstreamReq.Header, "X-Stainless-Lang"))
	require.Equal(t, "0.11.0", getHeaderRaw(upstreamReq.Header, "X-Stainless-Package-Version"))
}

func TestBuildCountTokensRequest_OAuth_FingerprintUnificationDisabledPreservesClientHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPost, "/v1/messages/count_tokens", nil)
	req.Header.Set("x-api-key", "test-key-count-tokens")
	req.Header.Set("User-Agent", "claude-cli/2.1.2 (client, cli)")
	req.Header.Set("X-Stainless-Lang", "count-client-js")
	c.Request = req

	cache := &recordingIdentityCache{fp: &Fingerprint{
		ClientID:                "cached-client-id",
		UserAgent:               "claude-cli/9.9.9 (cached, cli)",
		StainlessLang:           "cached-js",
		StainlessPackageVersion: "9.9.9",
		UpdatedAt:               time.Now().Unix(),
	}}
	svc := &GatewayService{
		identityService: NewIdentityService(cache),
		settingService: newGatewayForwardingSettingService(t, map[string]string{
			SettingKeyEnableFingerprintUnification: "false",
		}),
	}
	account := &Account{
		ID:       4245,
		Platform: PlatformAnthropic,
		Type:     AccountTypeOAuth,
	}
	body := []byte(`{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]}`)

	upstreamReq, err := svc.buildCountTokensRequest(context.Background(), c, account, body, "oauth-token", "oauth", "claude-sonnet-4-6", false)
	require.NoError(t, err)

	require.Equal(t, int64(0), cache.gets.Load(), "fingerprint cache should not be read for count_tokens when unification is disabled")
	require.Equal(t, "claude-cli/2.1.2 (client, cli)", getHeaderRaw(upstreamReq.Header, "User-Agent"))
	require.Equal(t, "count-client-js", getHeaderRaw(upstreamReq.Header, "X-Stainless-Lang"))
}

func TestClaudeCodeSessionCachePrunesToBoundedSize(t *testing.T) {
	claudeCodeSessionMu.Lock()
	claudeCodeSessionCache = map[string]claudeCodeSessionEntry{}
	claudeCodeSessionMu.Unlock()

	for i := 0; i < 4100; i++ {
		require.NotEmpty(t, getOrCreateClaudeCodeSessionID("api-key-hash-"+strconv.Itoa(i)))
	}

	claudeCodeSessionMu.Lock()
	size := len(claudeCodeSessionCache)
	claudeCodeSessionMu.Unlock()
	require.LessOrEqual(t, size, 4096)
}

func TestBuildUpstreamRequest_OAuth_SessionStablePerAPIKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	account := &Account{
		ID:       5001,
		Platform: PlatformAnthropic,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			"account_uuid": "acc-uuid-sticky",
		},
	}
	svc := &GatewayService{}
	body := []byte(`{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]}`)

	makeReq := func(apiKey string) *http.Request {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		r := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
		r.Header.Set("x-api-key", apiKey)
		c.Request = r
		req, err := svc.buildUpstreamRequest(context.Background(), c, account, body, "oauth-token", "oauth", "claude-sonnet-4-6", false, false)
		require.NoError(t, err)
		return req
	}

	reqA1 := makeReq("stable-api-key-a")
	reqA2 := makeReq("stable-api-key-a")
	reqB := makeReq("stable-api-key-b")

	sessionA1 := getHeaderRaw(reqA1.Header, "X-Claude-Code-Session-Id")
	sessionA2 := getHeaderRaw(reqA2.Header, "X-Claude-Code-Session-Id")
	sessionB := getHeaderRaw(reqB.Header, "X-Claude-Code-Session-Id")

	require.NotEmpty(t, sessionA1)
	require.Equal(t, sessionA1, sessionA2)
	require.NotEqual(t, sessionA1, sessionB)
}
