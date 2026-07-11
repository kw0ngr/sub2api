package admin

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type grokOAuthClientStub struct {
	refresh func(ctx context.Context, refreshToken, proxyURL, clientID string) (*xai.TokenResponse, error)
}

func (s grokOAuthClientStub) ExchangeCode(ctx context.Context, code, codeVerifier, redirectURI, proxyURL, clientID string) (*xai.TokenResponse, error) {
	return nil, errors.New("not implemented")
}

func (s grokOAuthClientStub) RefreshToken(ctx context.Context, refreshToken, proxyURL, clientID string) (*xai.TokenResponse, error) {
	return s.refresh(ctx, refreshToken, proxyURL, clientID)
}


// grokImportHTTPUpstreamStub is a minimal HTTPUpstream for AT import validation probes.
type grokImportHTTPUpstreamStub struct {
	status int
	body   string
	err    error
	last   *http.Request
}

func (u *grokImportHTTPUpstreamStub) Do(req *http.Request, proxyURL string, accountID int64, accountConcurrency int) (*http.Response, error) {
	u.last = req
	if u.err != nil {
		return nil, u.err
	}
	status := u.status
	if status == 0 {
		status = http.StatusOK
	}
	body := u.body
	if body == "" {
		body = `{"data":[]}`
	}
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}, nil
}

func (u *grokImportHTTPUpstreamStub) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, accountConcurrency)
}

func newGrokImportQuotaService(upstream service.HTTPUpstream) *service.GrokQuotaService {
	return service.NewGrokQuotaService(nil, nil, nil, upstream)
}

func TestGrokImportRefreshTokensCreatesAccounts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminSvc := newStubAdminService()
	oauthSvc := service.NewGrokOAuthService(nil, grokOAuthClientStub{
		refresh: func(ctx context.Context, refreshToken, proxyURL, clientID string) (*xai.TokenResponse, error) {
			return &xai.TokenResponse{
				AccessToken:  "access-" + refreshToken,
				RefreshToken: refreshToken,
				TokenType:    "Bearer",
				ExpiresIn:    3600,
			}, nil
		},
	})
	handler := NewGrokOAuthHandler(oauthSvc, adminSvc, nil)
	router := gin.New()
	router.POST("/admin/grok/oauth/import-refresh-tokens", handler.ImportRefreshTokens)
	payload := []byte(`{
		"raw_text":"rt-one\n# comment\nrt-two\nrt-one",
		"name_prefix":"Grok RT",
		"group_ids":[2],
		"concurrency":7,
		"priority":3,
		"model_mapping":{"grok":"grok-4.5"},
		"confirm_mixed_channel_risk":true
	}`)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/grok/oauth/import-refresh-tokens", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var envelope struct {
		Data GrokImportRefreshTokensResult `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if envelope.Data.Total != 2 || envelope.Data.Created != 2 || envelope.Data.Failed != 0 {
		t.Fatalf("result = %+v, want total=2 created=2 failed=0", envelope.Data)
	}
	if len(adminSvc.createdAccounts) != 2 {
		t.Fatalf("created accounts = %d, want 2", len(adminSvc.createdAccounts))
	}
	byRT := map[string]*service.CreateAccountInput{}
	for _, acc := range adminSvc.createdAccounts {
		rt, _ := acc.Credentials["refresh_token"].(string)
		byRT[rt] = acc
	}
	first, ok := byRT["rt-one"]
	if !ok {
		t.Fatalf("missing rt-one account: %+v", adminSvc.createdAccounts)
	}
	if first.Platform != service.PlatformGrok || first.Type != service.AccountTypeOAuth {
		t.Fatalf("created account platform/type = %s/%s", first.Platform, first.Type)
	}
	if got := first.Name; got != "Grok RT #1" {
		t.Fatalf("created account name = %q, want Grok RT #1", got)
	}
	if got := first.Credentials["base_url"]; got != xai.DefaultBaseURL {
		t.Fatalf("base_url = %v, want %s", got, xai.DefaultBaseURL)
	}
	if _, ok := first.Credentials["model_mapping"]; !ok {
		t.Fatal("model_mapping missing from credentials")
	}
	if !first.SkipMixedChannelCheck {
		t.Fatal("SkipMixedChannelCheck = false, want true")
	}
	if _, ok := byRT["rt-two"]; !ok {
		t.Fatal("missing rt-two account")
	}
}

func TestGrokImportRefreshTokensReturnsPartialFailures(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminSvc := newStubAdminService()
	oauthSvc := service.NewGrokOAuthService(nil, grokOAuthClientStub{
		refresh: func(ctx context.Context, refreshToken, proxyURL, clientID string) (*xai.TokenResponse, error) {
			if refreshToken == "bad" {
				return nil, errors.New("invalid refresh token")
			}
			return &xai.TokenResponse{AccessToken: "access-ok", RefreshToken: refreshToken, ExpiresIn: 3600}, nil
		},
	})
	handler := NewGrokOAuthHandler(oauthSvc, adminSvc, nil)
	router := gin.New()
	router.POST("/admin/grok/oauth/import-refresh-tokens", handler.ImportRefreshTokens)
	payload := []byte(`{"refresh_tokens":["ok","bad"],"name_prefix":"Grok"}`)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/grok/oauth/import-refresh-tokens", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var envelope struct {
		Data GrokImportRefreshTokensResult `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if envelope.Data.Created != 1 || envelope.Data.Failed != 1 {
		t.Fatalf("result = %+v, want created=1 failed=1", envelope.Data)
	}
	if len(adminSvc.createdAccounts) != 1 {
		t.Fatalf("created accounts = %d, want 1", len(adminSvc.createdAccounts))
	}
}

func TestGrokImportRefreshTokensRequiresTokens(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminSvc := newStubAdminService()
	oauthSvc := service.NewGrokOAuthService(nil, grokOAuthClientStub{
		refresh: func(ctx context.Context, refreshToken, proxyURL, clientID string) (*xai.TokenResponse, error) {
			return nil, errors.New("should not be called")
		},
	})
	handler := NewGrokOAuthHandler(oauthSvc, adminSvc, nil)
	router := gin.New()
	router.POST("/admin/grok/oauth/import-refresh-tokens", handler.ImportRefreshTokens)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/grok/oauth/import-refresh-tokens", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestSanitizeGrokImportTokenStripsSSOPrefixAndNoise(t *testing.T) {
	got := sanitizeGrokImportToken("  sso=\u200brt\u2013one\u00a0  ")
	if got != "sso=rt-one" {
		t.Fatalf("sanitize = %q, want sso=rt-one", got)
	}
	kind, token := detectGrokImportTokenKind(got, "auto")
	if kind != grokImportKindAccess || token != "rt-one" {
		t.Fatalf("detect = %s/%q", kind, token)
	}
}

func TestParseGrokImportTokenLinesDetectsKinds(t *testing.T) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"ES256","typ":"at+jwt"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"email":"a@x.ai","exp":` + strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10) + `}`))
	jwt := header + "." + payload + ".sig"

	lines := parseGrokImportTokenLines(GrokImportRefreshTokensRequest{
		RawText: "refresh_token:rt-1\naccess_token:" + jwt + "\nsso=cookie-token\n" + jwt,
	})
	// JWT line is deduped with explicit access_token line.
	if len(lines) != 3 {
		t.Fatalf("lines=%d want 3: %+v", len(lines), lines)
	}
	if lines[0].kind != grokImportKindRefresh || lines[0].token != "rt-1" {
		t.Fatalf("line0 = %+v", lines[0])
	}
	if lines[1].kind != grokImportKindAccess {
		t.Fatalf("line1 kind = %s", lines[1].kind)
	}
	if lines[2].kind != grokImportKindAccess || lines[2].token != "cookie-token" {
		t.Fatalf("line2 = %+v", lines[2])
	}
}

func TestGrokImportAccessTokensCreatesAccounts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminSvc := newStubAdminService()
	oauthSvc := service.NewGrokOAuthService(nil, grokOAuthClientStub{
		refresh: func(ctx context.Context, refreshToken, proxyURL, clientID string) (*xai.TokenResponse, error) {
			return nil, errors.New("should not refresh for access token import")
		},
	})
	upstream := &grokImportHTTPUpstreamStub{status: http.StatusOK}
	handler := NewGrokOAuthHandler(oauthSvc, adminSvc, newGrokImportQuotaService(upstream))
	router := gin.New()
	router.POST("/admin/grok/oauth/import-refresh-tokens", handler.ImportRefreshTokens)

	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"ES256","typ":"at+jwt"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"email":"user@x.ai","exp":` + strconv.FormatInt(time.Now().Add(2*time.Hour).Unix(), 10) + `}`))
	jwt := header + "." + payload + ".sig"

	body, _ := json.Marshal(map[string]any{
		"access_tokens": []string{jwt},
		"name_prefix":   "Grok AT",
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/grok/oauth/import-refresh-tokens", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var envelope struct {
		Data GrokImportRefreshTokensResult `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if envelope.Data.Created != 1 || envelope.Data.Failed != 0 {
		t.Fatalf("result=%+v", envelope.Data)
	}
	if len(adminSvc.createdAccounts) != 1 {
		t.Fatalf("created=%d", len(adminSvc.createdAccounts))
	}
	acc := adminSvc.createdAccounts[0]
	if acc.Credentials["access_token"] != jwt {
		t.Fatalf("access_token mismatch: %v", acc.Credentials["access_token"])
	}
	if _, ok := acc.Credentials["refresh_token"]; ok {
		t.Fatal("refresh_token should be absent for access-token-only import")
	}
	if acc.AutoPauseOnExpired == nil || !*acc.AutoPauseOnExpired {
		t.Fatalf("AutoPauseOnExpired = %v, want true", acc.AutoPauseOnExpired)
	}
	if !strings.Contains(envelope.Data.Results[0].Warning, "access_token only") {
		t.Fatalf("warning = %q", envelope.Data.Results[0].Warning)
	}
	if upstream.last == nil || !strings.Contains(upstream.last.URL.String(), "/models") {
		t.Fatalf("expected upstream /models validation probe, got %#v", upstream.last)
	}
	if auth := upstream.last.Header.Get("Authorization"); auth != "Bearer "+jwt {
		t.Fatalf("Authorization = %q", auth)
	}
}

func TestIsGrokAPIAccessTokenJWT(t *testing.T) {
	h := base64.RawURLEncoding.EncodeToString([]byte(`{"typ":"at+jwt","alg":"ES256"}`))
	if !isGrokAPIAccessTokenJWT(h + ".payload.sig") {
		t.Fatal("expected at+jwt to be accepted")
	}
	h2 := base64.RawURLEncoding.EncodeToString([]byte(`{"typ":"JWT","alg":"HS256"}`))
	if isGrokAPIAccessTokenJWT(h2 + ".payload.sig") {
		t.Fatal("expected SSO JWT to be rejected")
	}
}


func TestGrokImportRejectsSSOCookieJWT(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminSvc := newStubAdminService()
	oauthSvc := service.NewGrokOAuthService(nil, grokOAuthClientStub{
		refresh: func(ctx context.Context, refreshToken, proxyURL, clientID string) (*xai.TokenResponse, error) {
			return nil, errors.New("should not refresh for SSO cookie import")
		},
	})
	handler := NewGrokOAuthHandler(oauthSvc, adminSvc, nil)
	router := gin.New()
	router.POST("/admin/grok/oauth/import-refresh-tokens", handler.ImportRefreshTokens)

	header := base64.RawURLEncoding.EncodeToString([]byte(`{"typ":"JWT","alg":"HS256"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"email":"sso@x.ai","exp":` + strconv.FormatInt(time.Now().Add(2*time.Hour).Unix(), 10) + `}`))
	jwt := header + "." + payload + ".sig"

	body, _ := json.Marshal(map[string]any{
		"raw_text":    "sso=" + jwt,
		"name_prefix": "sso-test",
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/grok/oauth/import-refresh-tokens", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var envelope struct {
		Data GrokImportRefreshTokensResult `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode: %v body=%s", err, rec.Body.String())
	}
	if envelope.Data.Created != 0 || envelope.Data.Failed != 1 {
		t.Fatalf("created=%d failed=%d want created=0 failed=1 body=%s", envelope.Data.Created, envelope.Data.Failed, rec.Body.String())
	}
	if len(adminSvc.createdAccounts) != 0 {
		t.Fatalf("should not create accounts for SSO cookies, got %d", len(adminSvc.createdAccounts))
	}
	if envelope.Data.Results[0].Error == "" || !strings.Contains(envelope.Data.Results[0].Error, "typ=at+jwt") {
		t.Fatalf("error = %q", envelope.Data.Results[0].Error)
	}
}

func TestGrokImportAccessTokenRejectsUpstreamUnauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminSvc := newStubAdminService()
	oauthSvc := service.NewGrokOAuthService(nil, grokOAuthClientStub{
		refresh: func(ctx context.Context, refreshToken, proxyURL, clientID string) (*xai.TokenResponse, error) {
			return nil, errors.New("should not refresh")
		},
	})
	upstream := &grokImportHTTPUpstreamStub{
		status: http.StatusUnauthorized,
		body:   `{"code":"invalid-argument","error":"Incorrect API key provided"}`,
	}
	handler := NewGrokOAuthHandler(oauthSvc, adminSvc, newGrokImportQuotaService(upstream))
	router := gin.New()
	router.POST("/admin/grok/oauth/import-refresh-tokens", handler.ImportRefreshTokens)

	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"ES256","typ":"at+jwt"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"email":"fake@x.ai","exp":` + strconv.FormatInt(time.Now().Add(2*time.Hour).Unix(), 10) + `}`))
	jwt := header + "." + payload + ".sig"

	body, _ := json.Marshal(map[string]any{
		"access_tokens": []string{jwt},
		"name_prefix":   "Grok AT fake",
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/grok/oauth/import-refresh-tokens", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var envelope struct {
		Data GrokImportRefreshTokensResult `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if envelope.Data.Created != 0 || envelope.Data.Failed != 1 {
		t.Fatalf("result=%+v want created=0 failed=1", envelope.Data)
	}
	if len(adminSvc.createdAccounts) != 0 {
		t.Fatalf("should not create account, got %d", len(adminSvc.createdAccounts))
	}
	if !strings.Contains(envelope.Data.Results[0].Error, "rejected access_token") {
		t.Fatalf("error = %q", envelope.Data.Results[0].Error)
	}
}

func TestGrokImportAccessTokenRequiresValidator(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminSvc := newStubAdminService()
	oauthSvc := service.NewGrokOAuthService(nil, grokOAuthClientStub{
		refresh: func(ctx context.Context, refreshToken, proxyURL, clientID string) (*xai.TokenResponse, error) {
			return nil, errors.New("should not refresh")
		},
	})
	// quotaService nil => validation not configured => import fails, no account
	handler := NewGrokOAuthHandler(oauthSvc, adminSvc, nil)
	router := gin.New()
	router.POST("/admin/grok/oauth/import-refresh-tokens", handler.ImportRefreshTokens)

	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"ES256","typ":"at+jwt"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"email":"user@x.ai","exp":` + strconv.FormatInt(time.Now().Add(2*time.Hour).Unix(), 10) + `}`))
	jwt := header + "." + payload + ".sig"
	body, _ := json.Marshal(map[string]any{"access_tokens": []string{jwt}})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/grok/oauth/import-refresh-tokens", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	var envelope struct {
		Data GrokImportRefreshTokensResult `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &envelope)
	if envelope.Data.Created != 0 || len(adminSvc.createdAccounts) != 0 {
		t.Fatalf("created without validator: %+v accounts=%d", envelope.Data, len(adminSvc.createdAccounts))
	}
	if !strings.Contains(envelope.Data.Results[0].Error, "not configured") {
		t.Fatalf("error = %q", envelope.Data.Results[0].Error)
	}
}

