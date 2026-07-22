package admin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"golang.org/x/net/http/httpguts"
	"golang.org/x/sync/errgroup"
)

const (
	grokImportMaxConcurrency     = 20
	grokImportDefaultConcurrency = 5
	grokImportClockSkewSeconds   = 120
)

type GrokImportRefreshTokensRequest struct {
	RefreshTokens           []string          `json:"refresh_tokens"`
	AccessTokens            []string          `json:"access_tokens"`
	RawText                 string            `json:"raw_text"`
	ClientID                string            `json:"client_id"`
	ProxyID                 *int64            `json:"proxy_id"`
	NamePrefix              string            `json:"name_prefix"`
	Notes                   *string           `json:"notes"`
	GroupIDs                []int64           `json:"group_ids"`
	Concurrency             int               `json:"concurrency"`
	ImportConcurrency       int               `json:"import_concurrency"`
	Priority                int               `json:"priority"`
	RateMultiplier          *float64          `json:"rate_multiplier"`
	LoadFactor              *int              `json:"load_factor"`
	ModelMapping            map[string]any    `json:"model_mapping"`
	BaseURL                 string            `json:"base_url"`
	Headers                 map[string]string `json:"headers"`
	Extra                   map[string]any    `json:"extra"`
	ExpiresAt               *int64            `json:"expires_at"`
	AutoPauseOnExpired      *bool             `json:"auto_pause_on_expired"`
	ConfirmMixedChannelRisk bool              `json:"confirm_mixed_channel_risk"`
	// ImportMode: auto | refresh_token | access_token
	ImportMode string `json:"import_mode"`
}

type GrokImportRefreshTokenLineResult struct {
	Line         int                           `json:"line"`
	TokenPreview string                        `json:"token_preview,omitempty"`
	Kind         string                        `json:"kind,omitempty"`
	AccountID    int64                         `json:"account_id,omitempty"`
	Account      *dto.Account                  `json:"account,omitempty"`
	Email        string                        `json:"email,omitempty"`
	Created      bool                          `json:"created"`
	Skipped      bool                          `json:"skipped,omitempty"`
	ProbeResult  *service.GrokQuotaProbeResult `json:"probe_result,omitempty"`
	Warning      string                        `json:"warning,omitempty"`
	Error        string                        `json:"error,omitempty"`
}

type GrokImportRefreshTokensResult struct {
	Total   int                                `json:"total"`
	Created int                                `json:"created"`
	Failed  int                                `json:"failed"`
	Skipped int                                `json:"skipped"`
	Results []GrokImportRefreshTokenLineResult `json:"results"`
}

type grokImportTokenKind string

const (
	grokImportKindRefresh grokImportTokenKind = "refresh_token"
	grokImportKindAccess  grokImportTokenKind = "access_token"
	grokImportKindSSO     grokImportTokenKind = "sso"
)

type grokImportTokenLine struct {
	line  int
	token string
	kind  grokImportTokenKind
}

type grokImportAccountNameInput struct {
	prefix string
	email  string
	index  int
	multi  bool
}

func resolveGrokImportBaseURL(req GrokImportRefreshTokensRequest) (string, error) {
	raw := strings.TrimSpace(req.BaseURL)
	if raw == "" && req.Extra != nil {
		if desired, exists := req.Extra["desired_base_url"]; exists && desired != nil {
			value, ok := desired.(string)
			if !ok {
				return "", fmt.Errorf("extra.desired_base_url must be a string")
			}
			raw = strings.TrimSpace(value)
		}
	}
	return xai.ValidatedBaseURL(raw)
}

func normalizeGrokImportHeaders(headers map[string]string) (map[string]string, error) {
	if len(headers) == 0 {
		return nil, nil
	}
	normalized := make(map[string]string, len(headers))
	for rawName, rawValue := range headers {
		name := strings.TrimSpace(rawName)
		value := strings.TrimSpace(rawValue)
		if name == "" || value == "" {
			continue
		}
		if !httpguts.ValidHeaderFieldName(name) {
			return nil, fmt.Errorf("invalid header name %q", name)
		}
		if !httpguts.ValidHeaderFieldValue(value) {
			return nil, fmt.Errorf("invalid value for header %q", name)
		}
		switch strings.ToLower(name) {
		case "authorization", "host", "content-length", "content-type", "transfer-encoding", "connection":
			return nil, fmt.Errorf("reserved header %q cannot be overridden", name)
		}
		normalized[http.CanonicalHeaderKey(name)] = value
	}
	if len(normalized) == 0 {
		return nil, nil
	}
	return normalized, nil
}

func applyGrokImportCredentialOverrides(credentials map[string]any, req GrokImportRefreshTokensRequest) {
	if credentials == nil {
		return
	}
	credentials["base_url"] = req.BaseURL
	if len(req.Headers) > 0 {
		headers := make(map[string]string, len(req.Headers))
		for key, value := range req.Headers {
			headers[key] = value
		}
		credentials["headers"] = headers
	}
}

func (h *GrokOAuthHandler) ImportRefreshTokens(c *gin.Context) {
	var req GrokImportRefreshTokensRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	baseURL, err := resolveGrokImportBaseURL(req)
	if err != nil {
		response.BadRequest(c, "Invalid Grok base_url: "+err.Error())
		return
	}
	headers, err := normalizeGrokImportHeaders(req.Headers)
	if err != nil {
		response.BadRequest(c, "Invalid Grok headers: "+err.Error())
		return
	}
	req.BaseURL = baseURL
	req.Headers = headers
	lines := parseGrokImportTokenLines(req)
	if len(lines) == 0 {
		response.BadRequest(c, "refresh_tokens, access_tokens, sso tokens or raw_text is required")
		return
	}

	proxyURL := ""
	if req.ProxyID != nil {
		proxy, err := h.adminService.GetProxy(c.Request.Context(), *req.ProxyID)
		if err == nil && proxy != nil {
			proxyURL = proxy.URL()
		}
	}

	importConcurrency := req.ImportConcurrency
	if importConcurrency <= 0 {
		importConcurrency = grokImportDefaultConcurrency
	}
	if importConcurrency > grokImportMaxConcurrency {
		importConcurrency = grokImportMaxConcurrency
	}

	result := GrokImportRefreshTokensResult{
		Total:   len(lines),
		Results: make([]GrokImportRefreshTokenLineResult, len(lines)),
	}

	ctx := c.Request.Context()
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(importConcurrency)
	var mu sync.Mutex

	for i, line := range lines {
		index := i
		jobLine := line
		g.Go(func() error {
			if err := gctx.Err(); err != nil {
				return nil
			}
			lineResult := h.importGrokTokenLine(ctx, req, jobLine, proxyURL, index+1, len(lines) > 1)
			mu.Lock()
			result.Results[index] = lineResult
			if lineResult.Created {
				result.Created++
			} else if lineResult.Skipped {
				result.Skipped++
			} else {
				result.Failed++
			}
			mu.Unlock()
			return nil
		})
	}
	_ = g.Wait()
	response.Success(c, result)
}

func parseGrokImportTokenLines(req GrokImportRefreshTokensRequest) []grokImportTokenLine {
	mode := strings.ToLower(strings.TrimSpace(req.ImportMode))
	if mode == "" {
		mode = "auto"
	}

	lines := make([]grokImportTokenLine, 0, len(req.RefreshTokens)+len(req.AccessTokens)+8)
	seen := map[string]struct{}{}

	add := func(line int, raw string, forcedKind grokImportTokenKind) {
		token := sanitizeGrokImportToken(raw)
		if token == "" || strings.HasPrefix(token, "#") || strings.HasPrefix(token, "//") {
			return
		}
		kind := forcedKind
		if kind == "" {
			kind, token = detectGrokImportTokenKind(token, mode)
		}
		if token == "" {
			return
		}
		dedupeKey := string(kind) + "|" + token
		if _, ok := seen[dedupeKey]; ok {
			return
		}
		seen[dedupeKey] = struct{}{}
		lines = append(lines, grokImportTokenLine{line: line, token: token, kind: kind})
	}

	for lineNo, token := range strings.Split(req.RawText, "\n") {
		add(lineNo+1, token, "")
	}
	baseLine := len(strings.Split(req.RawText, "\n"))
	if strings.TrimSpace(req.RawText) == "" {
		baseLine = 0
	}
	for i, token := range req.RefreshTokens {
		add(baseLine+i+1, token, grokImportKindRefresh)
	}
	baseLine += len(req.RefreshTokens)
	for i, token := range req.AccessTokens {
		add(baseLine+i+1, token, grokImportKindAccess)
	}
	return lines
}

func detectGrokImportTokenKind(token, mode string) (grokImportTokenKind, string) {
	lower := strings.ToLower(token)
	// Support email|password|sso dump lines: take 3rd field as SSO when present.
	if strings.Count(token, "|") >= 2 {
		parts := strings.Split(token, "|")
		if len(parts) >= 3 {
			sso := strings.TrimSpace(parts[2])
			if sso != "" {
				return grokImportKindSSO, sso
			}
		}
	}
	if strings.Count(token, "----") >= 2 {
		parts := strings.Split(token, "----")
		if len(parts) >= 3 {
			sso := strings.TrimSpace(parts[2])
			if sso != "" {
				return grokImportKindSSO, sso
			}
		}
	}
	for _, prefix := range []string{
		"refresh_token:", "refresh_token=", "rt:", "rt=",
		"access_token:", "access_token=", "at:", "at=",
		"sso:", "sso=",
	} {
		if strings.HasPrefix(lower, prefix) {
			value := strings.TrimSpace(token[len(prefix):])
			switch {
			case strings.HasPrefix(prefix, "refresh") || strings.HasPrefix(prefix, "rt"):
				return grokImportKindRefresh, value
			case strings.HasPrefix(prefix, "sso"):
				return grokImportKindSSO, value
			default:
				return grokImportKindAccess, value
			}
		}
	}

	switch mode {
	case "refresh_token", "refresh", "rt":
		return grokImportKindRefresh, token
	case "access_token", "access", "at":
		return grokImportKindAccess, token
	case "sso":
		return grokImportKindSSO, token
	default:
		// HS256 typ=JWT web SSO cookies look like JWTs but are not OAuth ATs.
		if looksLikeJWT(token) {
			if isGrokAPIAccessTokenJWT(token) {
				return grokImportKindAccess, token
			}
			// Non-at+jwt JWT: treat as SSO candidate (will convert or fail clearly).
			return grokImportKindSSO, token
		}
		return grokImportKindRefresh, token
	}
}

func looksLikeJWT(token string) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return false
	}
	for _, part := range parts {
		if part == "" {
			return false
		}
	}
	header, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}
	var obj map[string]any
	if err := json.Unmarshal(header, &obj); err != nil {
		return false
	}
	_, hasAlg := obj["alg"]
	return hasAlg
}

func sanitizeGrokImportToken(value string) string {
	replacer := strings.NewReplacer(
		"\u2010", "-", "\u2011", "-", "\u2012", "-",
		"\u2013", "-", "\u2014", "-", "\u2212", "-",
		"\u00a0", " ", "\u2007", " ", "\u202f", " ",
		"\u200b", "", "\u200c", "", "\u200d", "", "\ufeff", "",
	)
	tok := replacer.Replace(strings.TrimSpace(value))
	tok = strings.Join(strings.FieldsFunc(tok, unicode.IsSpace), "")
	var b strings.Builder
	b.Grow(len(tok))
	for _, r := range tok {
		if r <= unicode.MaxASCII && r >= 32 {
			_, _ = b.WriteRune(r)
		}
	}
	return b.String()
}

func (h *GrokOAuthHandler) importGrokTokenLine(
	ctx context.Context,
	req GrokImportRefreshTokensRequest,
	line grokImportTokenLine,
	proxyURL string,
	index int,
	multi bool,
) GrokImportRefreshTokenLineResult {
	result := GrokImportRefreshTokenLineResult{
		Line:         line.line,
		TokenPreview: previewGrokRefreshToken(line.token),
		Kind:         string(line.kind),
	}

	switch line.kind {
	case grokImportKindAccess:
		return h.importGrokAccessToken(ctx, req, line, proxyURL, index, multi, result)
	case grokImportKindSSO:
		return h.importGrokSSOToken(ctx, req, line, proxyURL, index, multi, result)
	default:
		return h.importGrokRefreshToken(ctx, req, line, proxyURL, index, multi, result)
	}
}

func (h *GrokOAuthHandler) importGrokRefreshToken(
	ctx context.Context,
	req GrokImportRefreshTokensRequest,
	line grokImportTokenLine,
	proxyURL string,
	index int,
	multi bool,
	result GrokImportRefreshTokenLineResult,
) GrokImportRefreshTokenLineResult {
	tokenInfo, err := h.grokOAuthService.RefreshToken(ctx, line.token, proxyURL, req.ClientID)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	credentials := h.grokOAuthService.BuildAccountCredentials(tokenInfo)
	applyGrokImportCredentialOverrides(credentials, req)
	if len(req.ModelMapping) > 0 {
		credentials["model_mapping"] = req.ModelMapping
	}
	concurrency := req.Concurrency
	if concurrency <= 0 {
		concurrency = 3
	}
	account, err := h.adminService.CreateAccount(ctx, &service.CreateAccountInput{
		Name: grokImportAccountName(grokImportAccountNameInput{
			prefix: req.NamePrefix,
			email:  tokenInfo.Email,
			index:  index,
			multi:  multi,
		}),
		Notes:                 req.Notes,
		Platform:              service.PlatformGrok,
		Type:                  service.AccountTypeOAuth,
		Credentials:           credentials,
		Extra:                 mergeGrokImportExtra(req.Extra, tokenInfo),
		ProxyID:               req.ProxyID,
		Concurrency:           concurrency,
		Priority:              req.Priority,
		RateMultiplier:        req.RateMultiplier,
		LoadFactor:            req.LoadFactor,
		GroupIDs:              req.GroupIDs,
		ExpiresAt:             req.ExpiresAt,
		AutoPauseOnExpired:    req.AutoPauseOnExpired,
		SkipMixedChannelCheck: req.ConfirmMixedChannelRisk,
	})
	if err != nil {
		result.Error = err.Error()
		return result
	}
	result.Created = true
	result.AccountID = account.ID
	result.Account = dto.AccountFromService(account)
	result.Email = tokenInfo.Email
	return h.finalizeGrokImportedAccountHealth(ctx, account, req.BaseURL, result)
}

func (h *GrokOAuthHandler) importGrokSSOToken(
	ctx context.Context,
	req GrokImportRefreshTokensRequest,
	line grokImportTokenLine,
	proxyURL string,
	index int,
	multi bool,
	result GrokImportRefreshTokenLineResult,
) GrokImportRefreshTokenLineResult {
	// Convert web SSO cookie -> official OAuth RT/AT, then reuse RT import path.
	tokenInfo, webQuota, err := service.ConvertGrokSSOToOAuth(ctx, line.token, req.ClientID, proxyURL)
	if err != nil {
		result.Error = "sso convert failed: " + err.Error()
		return result
	}
	if strings.TrimSpace(tokenInfo.RefreshToken) == "" {
		result.Error = "sso convert produced no refresh_token"
		return result
	}
	// Re-run as refresh_token import using converted RT.
	line.token = tokenInfo.RefreshToken
	line.kind = grokImportKindRefresh
	result.Kind = string(grokImportKindSSO)
	out := h.importGrokRefreshToken(ctx, req, line, proxyURL, index, multi, result)
	if out.Created && webQuota != nil {
		out.Warning = strings.TrimSpace(strings.Join([]string{
			out.Warning,
			fmt.Sprintf("converted from SSO; web rate-limits remaining=%s/%s window=%ss",
				int64PtrValue(webQuota.RemainingQueries),
				int64PtrValue(webQuota.TotalQueries),
				int64PtrValue(webQuota.WindowSizeSeconds),
			),
		}, "; "))
		// Persist web rate-limits snapshot into account extra when possible.
		if out.AccountID > 0 && h.adminService != nil {
			// Best-effort via Create already done; schedule probe still runs.
			_ = webQuota
		}
	}
	if out.Email == "" && tokenInfo.Email != "" {
		out.Email = tokenInfo.Email
	}
	return out
}

func int64PtrValue(p *int64) string {
	if p == nil {
		return "?"
	}
	return fmt.Sprintf("%d", *p)
}

func (h *GrokOAuthHandler) importGrokAccessToken(
	ctx context.Context,
	req GrokImportRefreshTokensRequest,
	line grokImportTokenLine,
	proxyURL string,
	index int,
	multi bool,
	result GrokImportRefreshTokenLineResult,
) GrokImportRefreshTokenLineResult {
	now := time.Now().UTC()
	if !isGrokAPIAccessTokenJWT(line.token) {
		result.Error = "only xAI OAuth credentials are accepted: refresh_token or access_token with JWT header typ=at+jwt (console/web SSO cookies are not supported for api.x.ai)"
		return result
	}
	email, expUnix, warning := parseGrokAccessTokenClaims(line.token, now)
	if warning != "" && strings.Contains(strings.ToLower(warning), "expired") {
		result.Error = warning
		return result
	}
	if err := h.validateGrokAccessTokenUpstream(ctx, line.token, req.BaseURL, proxyURL, req.Headers); err != nil {
		result.Error = err.Error()
		return result
	}

	expiresAt := now.Add(6 * time.Hour).UTC()
	if expUnix > 0 {
		expiresAt = time.Unix(expUnix, 0).UTC()
	}

	credentials := map[string]any{
		"access_token": line.token,
		"token_type":   "Bearer",
		"expires_at":   expiresAt.Format(time.RFC3339),
		"base_url":     req.BaseURL,
	}
	applyGrokImportCredentialOverrides(credentials, req)
	if clientID := strings.TrimSpace(req.ClientID); clientID != "" {
		credentials["client_id"] = clientID
	} else {
		credentials["client_id"] = xai.EffectiveClientID()
	}
	if email != "" {
		credentials["email"] = email
	}
	if len(req.ModelMapping) > 0 {
		credentials["model_mapping"] = req.ModelMapping
	}

	autoPause := true
	if req.AutoPauseOnExpired != nil {
		autoPause = *req.AutoPauseOnExpired
	}

	tokenInfo := &service.GrokTokenInfo{
		AccessToken: line.token,
		Email:       email,
		ExpiresAt:   expiresAt.Unix(),
		TokenType:   "Bearer",
		ClientID:    fmt.Sprint(credentials["client_id"]),
	}

	// Drive account-level ExpiresAt from JWT exp so AutoPauseOnExpired works for AT-only accounts.
	accountExpiresAt := req.ExpiresAt
	if accountExpiresAt == nil && expUnix > 0 {
		v := expUnix
		accountExpiresAt = &v
	}

	account, err := h.adminService.CreateAccount(ctx, &service.CreateAccountInput{
		Name: grokImportAccountName(grokImportAccountNameInput{
			prefix: req.NamePrefix,
			email:  email,
			index:  index,
			multi:  multi,
		}),
		Notes:                 req.Notes,
		Platform:              service.PlatformGrok,
		Type:                  service.AccountTypeOAuth,
		Credentials:           credentials,
		Extra:                 mergeGrokImportExtra(req.Extra, tokenInfo),
		ProxyID:               req.ProxyID,
		Concurrency:           req.Concurrency,
		Priority:              req.Priority,
		RateMultiplier:        req.RateMultiplier,
		LoadFactor:            req.LoadFactor,
		GroupIDs:              req.GroupIDs,
		ExpiresAt:             accountExpiresAt,
		AutoPauseOnExpired:    &autoPause,
		SkipMixedChannelCheck: req.ConfirmMixedChannelRisk,
	})
	if err != nil {
		result.Error = err.Error()
		return result
	}
	result.Created = true
	result.AccountID = account.ID
	result.Account = dto.AccountFromService(account)
	result.Email = email
	if warning != "" {
		result.Warning = warning
	} else {
		result.Warning = "access_token only; no refresh_token, cannot auto-renew after expiry"
	}
	return h.finalizeGrokImportedAccountHealth(ctx, account, req.BaseURL, result)
}

func parseGrokAccessTokenClaims(token string, now time.Time) (email string, expUnix int64, warning string) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return "", 0, "access_token is not a JWT; expiry cannot be verified"
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", 0, "access_token payload is not decodable; expiry cannot be verified"
	}
	var claims struct {
		Email string `json:"email"`
		Exp   int64  `json:"exp"`
		Sub   string `json:"sub"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", 0, "access_token claims are not JSON; expiry cannot be verified"
	}
	email = strings.TrimSpace(claims.Email)
	if email == "" {
		email = strings.TrimSpace(claims.Sub)
	}
	if claims.Exp > 0 {
		if now.Unix() > claims.Exp+grokImportClockSkewSeconds {
			return email, claims.Exp, fmt.Sprintf("access_token expired at %s", time.Unix(claims.Exp, 0).UTC().Format(time.RFC3339))
		}
		return email, claims.Exp, ""
	}
	return email, 0, "access_token has no exp claim; expiry cannot be verified"
}

func grokImportAccountName(input grokImportAccountNameInput) string {
	name := strings.TrimSpace(input.prefix)
	if name == "" {
		name = strings.TrimSpace(input.email)
	}
	if name == "" {
		name = "Grok OAuth Account"
	}
	if input.multi {
		return fmt.Sprintf("%s #%d", name, input.index)
	}
	return name
}

func mergeGrokImportExtra(extra map[string]any, tokenInfo *service.GrokTokenInfo) map[string]any {
	merged := map[string]any{}
	for key, value := range extra {
		merged[key] = value
	}
	if tokenInfo == nil {
		return merged
	}
	if tokenInfo.Email != "" {
		merged["email"] = tokenInfo.Email
	}
	if tokenInfo.SubscriptionTier != "" {
		merged["subscription_tier"] = tokenInfo.SubscriptionTier
	}
	if tokenInfo.EntitlementStatus != "" {
		merged["entitlement_status"] = tokenInfo.EntitlementStatus
	}
	return merged
}

func previewGrokRefreshToken(token string) string {
	token = strings.TrimSpace(token)
	if len(token) <= 12 {
		return "***"
	}
	return token[:6] + "…" + token[len(token)-4:]
}

func appendGrokImportWarning(existing, warning string) string {
	existing = strings.TrimSpace(existing)
	warning = strings.TrimSpace(warning)
	if warning == "" {
		return existing
	}
	if existing == "" {
		return warning
	}
	return existing + "; " + warning
}

// finalizeGrokImportedAccountHealth prevents CLI accounts from entering a
// production group based only on a successful token refresh or GET /models.
// They stay unschedulable until a real /responses probe succeeds.
func (h *GrokOAuthHandler) finalizeGrokImportedAccountHealth(
	ctx context.Context,
	account *service.Account,
	baseURL string,
	result GrokImportRefreshTokenLineResult,
) GrokImportRefreshTokenLineResult {
	if account == nil || account.ID <= 0 {
		return result
	}
	if !xai.IsCLIChatProxyBaseURL(baseURL) {
		h.scheduleGrokQuotaProbe(account.ID)
		return result
	}
	if h == nil || h.quotaService == nil || h.adminService == nil {
		result.Warning = appendGrokImportWarning(result.Warning, "CLI account imported without inference health gate: quota probe is not configured")
		return result
	}

	if pending, err := h.adminService.SetAccountSchedulable(ctx, account.ID, false); err != nil {
		result.Warning = appendGrokImportWarning(result.Warning, "failed to set pending health state: "+err.Error())
	} else if pending != nil {
		account = pending
		result.Account = dto.AccountFromService(pending)
	}

	probeCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	probe, err := h.quotaService.ProbeHeaders(probeCtx, account.ID)
	result.ProbeResult = probe
	if err != nil {
		result.Warning = appendGrokImportWarning(result.Warning, "CLI inference health probe failed: "+err.Error())
	} else if probe == nil {
		result.Warning = appendGrokImportWarning(result.Warning, "CLI inference health probe returned no result")
	} else {
		statusCode := probe.StatusCode
		detail := strings.TrimSpace(probe.ErrorMessage)
		switch {
		case statusCode == 0:
			if detail == "" {
				detail = "no authoritative inference result"
			}
			result.Warning = appendGrokImportWarning(result.Warning, "CLI inference health probe did not complete: "+detail)
		case statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices:
			if detail == "" {
				detail = http.StatusText(statusCode)
			}
			result.Warning = appendGrokImportWarning(
				result.Warning,
				fmt.Sprintf("CLI inference health probe rejected account (HTTP %d): %s", statusCode, detail),
			)
		}
	}

	if refreshed, getErr := h.adminService.GetAccount(ctx, account.ID); getErr == nil && refreshed != nil {
		result.Account = dto.AccountFromService(refreshed)
	}
	return result
}

func (h *GrokOAuthHandler) scheduleGrokQuotaProbe(accountID int64) {
	if h == nil || h.quotaService == nil || accountID <= 0 {
		return
	}
	go func(id int64) {
		defer func() { _ = recover() }()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, _ = h.quotaService.ProbeUsage(ctx, id)
	}(accountID)
}

// validateGrokAccessTokenUpstream probes the selected xAI upstream with the
// candidate access token before creating an account. Without this, any forged
// typ=at+jwt JWT would import.
func (h *GrokOAuthHandler) validateGrokAccessTokenUpstream(ctx context.Context, accessToken, baseURL, proxyURL string, headers map[string]string) error {
	if h == nil || h.quotaService == nil {
		return fmt.Errorf("access_token upstream validation is not configured")
	}
	return h.quotaService.ValidateAccessToken(ctx, accessToken, baseURL, proxyURL, headers)
}

// isGrokAPIAccessTokenJWT reports whether token looks like an xAI OAuth access token
// (JWT header typ=at+jwt). Console/web SSO cookies use typ=JWT + HS256 and cannot
// authenticate against api.x.ai as Bearer tokens.
func isGrokAPIAccessTokenJWT(token string) bool {
	parts := strings.Split(token, ".")
	if len(parts) < 2 || parts[0] == "" {
		return false
	}
	raw := parts[0]
	header, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		switch len(raw) % 4 {
		case 2:
			raw += "=="
		case 3:
			raw += "="
		}
		header, err = base64.URLEncoding.DecodeString(raw)
		if err != nil {
			return false
		}
	}
	var obj struct {
		Typ string `json:"typ"`
	}
	if err := json.Unmarshal(header, &obj); err != nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(obj.Typ), "at+jwt")
}
