package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"golang.org/x/net/publicsuffix"
)

const (
	grokSSOWarmURL          = "https://accounts.x.ai/sign-in?redirect=grok-com&email=true"
	grokSSOConsentUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36"
	grokWebRateLimitsURL    = "https://grok.com/rest/rate-limits"
	grokWebRateLimitsExtra  = "grok_web_rate_limits"
)

var (
	grokHiddenInputRe = regexp.MustCompile(`(?i)<input[^>]+type="hidden"[^>]*>`)
	grokInputNameRe   = regexp.MustCompile(`(?i)name="([^"]+)"`)
	grokInputValueRe  = regexp.MustCompile(`(?i)value="([^"]*)"`)
	grokFormActionRe  = regexp.MustCompile(`(?i)<form[^>]+action="([^"]+)"`)
)

// GrokWebRateLimits is the grok.com/rest/rate-limits snapshot (SSO cookie path).
type GrokWebRateLimits struct {
	ModelName         string `json:"model_name,omitempty"`
	RemainingQueries  *int64 `json:"remaining_queries,omitempty"`
	TotalQueries      *int64 `json:"total_queries,omitempty"`
	WindowSizeSeconds *int64 `json:"window_size_seconds,omitempty"`
	ResetAt           string `json:"reset_at,omitempty"`
	SyncedAt          string `json:"synced_at,omitempty"`
	Source            string `json:"source,omitempty"`
	RawStatus         int    `json:"raw_status,omitempty"`
}

// ConvertGrokSSOToOAuth exchanges a grok.com/console SSO cookie for official
// api.x.ai OAuth tokens via pure HTTP (no browser):
//
//	warm accounts.x.ai session → PKCE authorize → consent decision=allow → token exchange
func ConvertGrokSSOToOAuth(ctx context.Context, ssoToken, clientID, proxyURL string) (*GrokTokenInfo, *GrokWebRateLimits, error) {
	ssoToken = stripGrokSSOPrefix(ssoToken)
	if strings.TrimSpace(ssoToken) == "" {
		return nil, nil, fmt.Errorf("sso token is empty")
	}
	if strings.TrimSpace(clientID) == "" {
		clientID = xai.EffectiveClientID()
	}

	// Best-effort web quota probe while SSO is still usable.
	webQuota, _ := ProbeGrokWebRateLimits(ctx, ssoToken, "fast", proxyURL)

	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		return nil, webQuota, fmt.Errorf("cookie jar: %w", err)
	}
	// Seed SSO cookies for .x.ai
	seedURL, _ := url.Parse("https://accounts.x.ai/")
	jar.SetCookies(seedURL, []*http.Cookie{
		{Name: "sso", Value: ssoToken, Path: "/", Domain: ".x.ai", Secure: true},
		{Name: "sso-rw", Value: ssoToken, Path: "/", Domain: ".x.ai", Secure: true},
	})

	client := &http.Client{
		Jar:     jar,
		Timeout: 45 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Capture localhost callback without following.
			if req.URL.Hostname() == "127.0.0.1" || req.URL.Hostname() == "localhost" {
				return http.ErrUseLastResponse
			}
			if len(via) >= 12 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}
	// proxyURL is currently unused for consent path; account proxy can be applied later if needed.
	_ = proxyURL

	// 1) Warm session
	if err := grokSSODo(ctx, client, http.MethodGet, grokSSOWarmURL, nil, map[string]string{
		"Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
	}); err != nil {
		// Non-fatal: some SSO still proceed to consent.
		_ = err
	}

	// 2) PKCE authorize
	codeVerifier, err := randomB64URL(32)
	if err != nil {
		return nil, webQuota, err
	}
	sum := sha256.Sum256([]byte(codeVerifier))
	codeChallenge := base64.RawURLEncoding.EncodeToString(sum[:])
	state, err := randomB64URL(16)
	if err != nil {
		return nil, webQuota, err
	}
	nonce, err := randomB64URL(8)
	if err != nil {
		return nil, webQuota, err
	}
	redirectURI := xai.EffectiveRedirectURI("")
	authURL, err := xai.BuildAuthorizationURL(state, codeChallenge, redirectURI, nonce)
	if err != nil {
		return nil, webQuota, err
	}
	// Ensure client_id override if custom
	if clientID != xai.EffectiveClientID() {
		u, parseErr := url.Parse(authURL)
		if parseErr == nil {
			q := u.Query()
			q.Set("client_id", clientID)
			u.RawQuery = q.Encode()
			authURL = u.String()
		}
	}

	status, body, finalURL, err := grokSSODoFull(ctx, client, http.MethodGet, authURL, nil, map[string]string{
		"Accept":  "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
		"Referer": "https://grok.com/",
	})
	if err != nil {
		return nil, webQuota, fmt.Errorf("authorize GET failed: %w", err)
	}
	html := string(body)

	var callback string
	if isLocalCallback(finalURL) && strings.Contains(finalURL, "code=") {
		callback = finalURL
	} else {
		fields := parseGrokHiddenInputs(html)
		if strings.TrimSpace(fields["client_id"]) == "" {
			return nil, webQuota, fmt.Errorf("consent form not found (status=%d url=%s)", status, truncateForErr(finalURL, 120))
		}
		action := "https://auth.x.ai/oauth2/authorize"
		if m := grokFormActionRe.FindStringSubmatch(html); len(m) == 2 && strings.TrimSpace(m[1]) != "" {
			action = m[1]
		}
		form := url.Values{}
		for k, v := range fields {
			form.Set(k, v)
		}
		if form.Get("response_type") == "" {
			form.Set("response_type", "code")
		}
		if form.Get("plan") == "" {
			form.Set("plan", "generic")
		}
		form.Set("decision", "allow")

		referer := finalURL
		if !strings.HasPrefix(referer, "http") {
			referer = authURL
		}
		status2, _, final2, err2 := grokSSODoFull(ctx, client, http.MethodPost, action, strings.NewReader(form.Encode()), map[string]string{
			"Content-Type": "application/x-www-form-urlencoded",
			"Origin":       "https://accounts.x.ai",
			"Referer":      referer,
			"Accept":       "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
		})
		if err2 != nil && !isLocalCallback(final2) {
			return nil, webQuota, fmt.Errorf("consent POST failed: %w", err2)
		}
		callback = final2
		if !strings.Contains(callback, "code=") {
			return nil, webQuota, fmt.Errorf("consent POST did not return code (status=%d final=%s)", status2, truncateForErr(final2, 160))
		}
	}

	u, err := url.Parse(callback)
	if err != nil {
		return nil, webQuota, fmt.Errorf("parse callback: %w", err)
	}
	code := strings.TrimSpace(u.Query().Get("code"))
	gotState := strings.TrimSpace(u.Query().Get("state"))
	if code == "" {
		return nil, webQuota, fmt.Errorf("callback missing code")
	}
	if gotState != "" && gotState != state {
		return nil, webQuota, fmt.Errorf("oauth state mismatch")
	}

	// 3) Token exchange
	tokenURL := xai.EffectiveTokenURL()
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", clientID)
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("code_verifier", codeVerifier)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, webQuota, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "sub2api-grok-sso-oauth/1.0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, webQuota, fmt.Errorf("token exchange: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 400 {
		return nil, webQuota, fmt.Errorf("token exchange HTTP %d: %s", resp.StatusCode, truncateForErr(string(raw), 240))
	}
	var tokenResp xai.TokenResponse
	if err := json.Unmarshal(raw, &tokenResp); err != nil {
		return nil, webQuota, fmt.Errorf("decode token response: %w", err)
	}
	if strings.TrimSpace(tokenResp.RefreshToken) == "" && strings.TrimSpace(tokenResp.AccessToken) == "" {
		return nil, webQuota, fmt.Errorf("token response missing credentials")
	}

	info := &GrokTokenInfo{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		IDToken:      tokenResp.IDToken,
		TokenType:    tokenResp.TokenType,
		ExpiresIn:    tokenResp.ExpiresIn,
		ClientID:     clientID,
		Scope:        tokenResp.Scope,
	}
	if info.ExpiresIn > 0 {
		info.ExpiresAt = time.Now().UTC().Add(time.Duration(info.ExpiresIn) * time.Second).Unix()
	}
	if email := extractEmailFromJWT(info.AccessToken); email != "" {
		info.Email = email
	}
	return info, webQuota, nil
}

// ProbeGrokWebRateLimits calls grok.com/rest/rate-limits with an SSO cookie
// (same endpoint grok2api uses).
func ProbeGrokWebRateLimits(ctx context.Context, ssoToken, modelName, proxyURL string) (*GrokWebRateLimits, error) {
	_ = proxyURL
	ssoToken = stripGrokSSOPrefix(ssoToken)
	if ssoToken == "" {
		return nil, fmt.Errorf("empty sso")
	}
	if strings.TrimSpace(modelName) == "" {
		modelName = "fast"
	}
	payload, _ := json.Marshal(map[string]string{"modelName": modelName})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, grokWebRateLimitsURL, strings.NewReader(string(payload)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Origin", "https://grok.com")
	req.Header.Set("Referer", "https://grok.com/")
	req.Header.Set("User-Agent", grokSSOConsentUserAgent)
	req.Header.Set("Cookie", "sso="+ssoToken+"; sso-rw="+ssoToken)

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	out := &GrokWebRateLimits{
		ModelName: modelName,
		SyncedAt:  time.Now().UTC().Format(time.RFC3339),
		Source:    "grok_web_rate_limits",
		RawStatus: resp.StatusCode,
	}
	if resp.StatusCode >= 400 {
		return out, fmt.Errorf("rate-limits HTTP %d: %s", resp.StatusCode, truncateForErr(string(raw), 160))
	}
	var body struct {
		WindowSizeSeconds *int64 `json:"windowSizeSeconds"`
		RemainingQueries  *int64 `json:"remainingQueries"`
		TotalQueries      *int64 `json:"totalQueries"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return out, err
	}
	out.WindowSizeSeconds = body.WindowSizeSeconds
	out.RemainingQueries = body.RemainingQueries
	out.TotalQueries = body.TotalQueries
	if body.WindowSizeSeconds != nil && *body.WindowSizeSeconds > 0 {
		out.ResetAt = time.Now().UTC().Add(time.Duration(*body.WindowSizeSeconds) * time.Second).Format(time.RFC3339)
	}
	return out, nil
}

func stripGrokSSOPrefix(token string) string {
	tok := strings.TrimSpace(token)
	// Prefer extracting sso= value from full cookie headers: "sso=abc; sso-rw=abc"
	if m := regexp.MustCompile(`(?i)(?:^|[;\s])sso=([^;]+)`).FindStringSubmatch(tok); len(m) == 2 {
		return strings.TrimSpace(m[1])
	}
	lower := strings.ToLower(tok)
	for _, prefix := range []string{"sso=", "sso:", "cookie:sso=", "cookie: sso="} {
		if strings.HasPrefix(lower, prefix) {
			return strings.TrimSpace(tok[len(prefix):])
		}
	}
	return tok
}

func parseGrokHiddenInputs(html string) map[string]string {
	out := map[string]string{}
	for _, tag := range grokHiddenInputRe.FindAllString(html, -1) {
		nameM := grokInputNameRe.FindStringSubmatch(tag)
		if len(nameM) != 2 {
			continue
		}
		val := ""
		if valM := grokInputValueRe.FindStringSubmatch(tag); len(valM) == 2 {
			val = valM[1]
		}
		out[nameM[1]] = val
	}
	return out
}

func grokSSODo(ctx context.Context, client *http.Client, method, rawURL string, body io.Reader, headers map[string]string) error {
	_, _, _, err := grokSSODoFull(ctx, client, method, rawURL, body, headers)
	return err
}

func grokSSODoFull(ctx context.Context, client *http.Client, method, rawURL string, body io.Reader, headers map[string]string) (int, []byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return 0, nil, "", err
	}
	req.Header.Set("User-Agent", grokSSOConsentUserAgent)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, "", err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	final := ""
	if resp.Request != nil && resp.Request.URL != nil {
		final = resp.Request.URL.String()
	}
	// When CheckRedirect returns ErrUseLastResponse, Location has the callback.
	if loc := resp.Header.Get("Location"); loc != "" && isLocalCallback(loc) {
		final = loc
	}
	if resp.StatusCode >= 400 && !isLocalCallback(final) {
		return resp.StatusCode, raw, final, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return resp.StatusCode, raw, final, nil
}

func isLocalCallback(u string) bool {
	return strings.HasPrefix(u, "http://127.0.0.1") || strings.HasPrefix(u, "http://localhost")
}

func randomB64URL(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func extractEmailFromJWT(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var claims struct {
		Email string `json:"email"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return ""
	}
	return strings.TrimSpace(claims.Email)
}

func truncateForErr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
