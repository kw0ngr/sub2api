package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestApplyGLMZCodeMimicHeaders(t *testing.T) {
	tests := []struct {
		name        string
		baseURL     string
		compatMode  string
		strong      bool
		wantApplied bool
		wantUA      string
	}{
		{"official bigmodel light", "https://open.bigmodel.cn/api/anthropic", GLMCompatModeAnthropic, false, true, "ZCode/unknown"},
		{"official z ai strong", "https://api.z.ai/api/anthropic", GLMCompatModeAnthropic, true, true, "ZCode/3.1.2"},
		{"empty base uses official default", "", GLMCompatModeAnthropic, false, true, "ZCode/unknown"},
		{"custom relay", "https://relay.example.test", GLMCompatModeAnthropic, true, false, "custom-client"},
		{"glm openai compat mode", "https://open.bigmodel.cn/api/anthropic", GLMCompatModeOpenAI, true, false, "custom-client"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqURL := "https://open.bigmodel.cn/api/anthropic/v1/messages?beta=true"
			if tt.baseURL != "" {
				reqURL = tt.baseURL + "/v1/messages?beta=true"
			}
			req := httptest.NewRequest(http.MethodPost, reqURL, nil)
			req.Header.Set("User-Agent", "custom-client")
			req.Header.Set("x-api-key", "sk-glm-test")
			req.Header.Set("content-type", "application/json")
			req.Header.Set("anthropic-version", "2023-06-01")

			applied := applyGLMZCodeMimicHeaders(req, newGLMAccount(0, tt.baseURL, tt.compatMode), tt.strong)

			require.Equal(t, tt.wantApplied, applied)
			require.Equal(t, "sk-glm-test", req.Header.Get("x-api-key"))
			require.Equal(t, "application/json", req.Header.Get("content-type"))
			require.Equal(t, "2023-06-01", req.Header.Get("anthropic-version"))
			require.Equal(t, tt.wantUA, req.Header.Get("User-Agent"))
			if !tt.wantApplied {
				require.Empty(t, getHeaderRaw(req.Header, "X-ZCode-Agent"))
				require.Empty(t, getHeaderRaw(req.Header, "X-ZCode-App-Version"))
				require.Empty(t, getHeaderRaw(req.Header, "X-Platform"))
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

func TestIsOfficialGLMAnthropicBaseURL(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want bool
	}{
		{"empty base uses default official anthropic base", "", true},
		{"official bigmodel anthropic base", "https://open.bigmodel.cn/api/anthropic", true},
		{"official bigmodel messages path", "https://open.bigmodel.cn/api/anthropic/v1/messages", true},
		{"official bigmodel messages path with trailing slash", "https://open.bigmodel.cn/api/anthropic/v1/messages/", true},
		{"official z ai anthropic base", "https://api.z.ai/api/anthropic", true},
		{"official z ai messages path", "https://api.z.ai/api/anthropic/v1/messages", true},
		{"bigmodel explicit custom port", "https://open.bigmodel.cn:8443/api/anthropic", false},
		{"z ai explicit custom port", "https://api.z.ai:8443/api/anthropic", false},
		{"bigmodel explicit default port", "https://open.bigmodel.cn:443/api/anthropic", false},
		{"uppercase official path", "https://open.bigmodel.cn/API/anthropic", false},
		{"relay custom domain", "https://relay.example.test/v1", false},
		{"host suffix spoofing", "https://open.bigmodel.cn.evil.test/api/anthropic", false},
		{"http scheme", "http://open.bigmodel.cn/api/anthropic", false},
		{"openai compatible coding endpoint", "https://open.bigmodel.cn/api/coding/paas/v4", false},
		{"parent api path is not anthropic", "https://open.bigmodel.cn/api", false},
		{"missing scheme", "open.bigmodel.cn/api/anthropic", false},
		{"invalid url", "https://%", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, isOfficialGLMAnthropicBaseURL(tt.raw))
		})
	}
}

func TestGLMZCodeMimicSkipsRelayAndSpoofedHosts(t *testing.T) {
	tests := []struct {
		name       string
		baseURL    string
		compatMode string
	}{
		{"custom relay", "https://relay.example.test/v1", GLMCompatModeAnthropic},
		{"bigmodel explicit custom port", "https://open.bigmodel.cn:8443/api/anthropic", GLMCompatModeAnthropic},
		{"z ai explicit custom port", "https://api.z.ai:8443/api/anthropic", GLMCompatModeAnthropic},
		{"host suffix spoofing", "https://open.bigmodel.cn.evil.test/api/anthropic", GLMCompatModeAnthropic},
		{"http scheme", "http://open.bigmodel.cn/api/anthropic", GLMCompatModeAnthropic},
		{"openai compatible coding endpoint", "https://open.bigmodel.cn/api/coding/paas/v4", GLMCompatModeAnthropic},
		{"uppercase official path", "https://open.bigmodel.cn/API/anthropic", GLMCompatModeAnthropic},
		{"glm openai compat mode", "https://open.bigmodel.cn/api/anthropic", GLMCompatModeOpenAI},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "https://relay.example.test/v1/messages", nil)
			req.Header.Set("User-Agent", "custom-client")
			req.Header.Set("x-api-key", "sk-glm-test")
			req.Header.Set("content-type", "application/json")
			req.Header.Set("anthropic-version", "2023-06-01")

			applied := applyGLMZCodeMimicHeaders(req, newGLMAccount(90000, tt.baseURL, tt.compatMode), true)

			require.False(t, applied)
			require.Equal(t, "custom-client", req.Header.Get("User-Agent"))
			require.Equal(t, "sk-glm-test", req.Header.Get("x-api-key"))
			require.Equal(t, "application/json", req.Header.Get("content-type"))
			require.Equal(t, "2023-06-01", req.Header.Get("anthropic-version"))
			require.Empty(t, getHeaderRaw(req.Header, "X-ZCode-Agent"))
			require.Empty(t, getHeaderRaw(req.Header, "X-ZCode-App-Version"))
			require.Empty(t, getHeaderRaw(req.Header, "X-Platform"))
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
			name:     "official z ai light",
			baseURL:  "https://api.z.ai/api/anthropic",
			clientUA: "curl/8",
			wantURL:  "https://api.z.ai/api/anthropic/v1/messages?beta=true",
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

func newGLMAccount(id int64, baseURL string, compatMode string) *Account {
	credentials := map[string]any{
		"api_key":     "sk-glm-test",
		"compat_mode": compatMode,
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
		newGLMAccount(accountID, baseURL, GLMCompatModeAnthropic),
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
