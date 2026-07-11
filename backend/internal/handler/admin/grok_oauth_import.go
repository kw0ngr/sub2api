package admin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"golang.org/x/sync/errgroup"
)

const (
	grokImportMaxConcurrency     = 20
	grokImportDefaultConcurrency = 5
	grokImportClockSkewSeconds   = 120
)

type GrokImportRefreshTokensRequest struct {
	RefreshTokens           []string       `json:"refresh_tokens"`
	AccessTokens            []string       `json:"access_tokens"`
	RawText                 string         `json:"raw_text"`
	ClientID                string         `json:"client_id"`
	ProxyID                 *int64         `json:"proxy_id"`
	NamePrefix              string         `json:"name_prefix"`
	Notes                   *string        `json:"notes"`
	GroupIDs                []int64        `json:"group_ids"`
	Concurrency             int            `json:"concurrency"`
	ImportConcurrency       int            `json:"import_concurrency"`
	Priority                int            `json:"priority"`
	RateMultiplier          *float64       `json:"rate_multiplier"`
	LoadFactor              *int           `json:"load_factor"`
	ModelMapping            map[string]any `json:"model_mapping"`
	Extra                   map[string]any `json:"extra"`
	ExpiresAt               *int64         `json:"expires_at"`
	AutoPauseOnExpired      *bool          `json:"auto_pause_on_expired"`
	ConfirmMixedChannelRisk bool           `json:"confirm_mixed_channel_risk"`
	// ImportMode: auto | refresh_token | access_token
	ImportMode string `json:"import_mode"`
}

type GrokImportRefreshTokenLineResult struct {
	Line         int          `json:"line"`
	TokenPreview string       `json:"token_preview,omitempty"`
	Kind         string       `json:"kind,omitempty"`
	AccountID    int64        `json:"account_id,omitempty"`
	Account      *dto.Account `json:"account,omitempty"`
	Email        string       `json:"email,omitempty"`
	Created      bool         `json:"created"`
	Skipped      bool         `json:"skipped,omitempty"`
	Warning      string       `json:"warning,omitempty"`
	Error        string       `json:"error,omitempty"`
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

func (h *GrokOAuthHandler) ImportRefreshTokens(c *gin.Context) {
	var req GrokImportRefreshTokensRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	lines := parseGrokImportTokenLines(req)
	if len(lines) == 0 {
		response.BadRequest(c, "refresh_tokens, access_tokens or raw_text is required")
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

	g, gctx := errgroup.WithContext(c.Request.Context())
	g.SetLimit(importConcurrency)
	var mu sync.Mutex

	for i, line := range lines {
		index := i
		jobLine := line
		g.Go(func() error {
			if err := gctx.Err(); err != nil {
				return nil
			}
			lineResult := h.importGrokTokenLine(c.Request.Context(), req, jobLine, proxyURL, index+1, len(lines) > 1)
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
			default:
				return grokImportKindAccess, value
			}
		}
	}

	switch mode {
	case "refresh_token", "refresh", "rt":
		return grokImportKindRefresh, token
	case "access_token", "access", "at", "sso":
		return grokImportKindAccess, token
	default:
		if looksLikeJWT(token) {
			return grokImportKindAccess, token
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
			b.WriteRune(r)
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
		return h.importGrokAccessToken(ctx, req, line, index, multi, result)
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
	if len(req.ModelMapping) > 0 {
		credentials["model_mapping"] = req.ModelMapping
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
		Concurrency:           req.Concurrency,
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
	h.scheduleGrokQuotaProbe(account.ID)
	return result
}

func (h *GrokOAuthHandler) importGrokAccessToken(
	ctx context.Context,
	req GrokImportRefreshTokensRequest,
	line grokImportTokenLine,
	index int,
	multi bool,
	result GrokImportRefreshTokenLineResult,
) GrokImportRefreshTokenLineResult {
	now := time.Now().UTC()
	email, expUnix, warning := parseGrokAccessTokenClaims(line.token, now)
	if warning != "" && strings.Contains(strings.ToLower(warning), "expired") {
		result.Error = warning
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
		"base_url":     xai.DefaultBaseURL,
	}
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
		ExpiresAt:             req.ExpiresAt,
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
	h.scheduleGrokQuotaProbe(account.ID)
	if warning != "" {
		result.Warning = warning
	} else {
		result.Warning = "access_token only; no refresh_token, cannot auto-renew after expiry"
	}
	return result
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
