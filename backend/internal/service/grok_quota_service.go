package service

import (
	"bytes"
	"fmt"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
)

const (
	grokQuotaUpstreamTimeout      = 20 * time.Second
	grokQuotaManagementBaseURL    = "https://management-api.x.ai"
	grokOfficialUsageExtraKey     = "grok_official_usage"
	grokOfficialUsageDefaultZone  = "Etc/GMT"
	grokOfficialUsageDefaultValue = "usd"
	grokOfficialUsageTokenValue   = "usage"
)

type GrokQuotaProbeResult struct {
	Source          string             `json:"source"`
	Snapshot        *xai.QuotaSnapshot `json:"snapshot,omitempty"`
	OfficialUsage   *xai.UsageSnapshot `json:"official_usage,omitempty"`
	StatusCode      int                `json:"status_code,omitempty"`
	ErrorMessage    string             `json:"error_message,omitempty"`
	HeadersObserved bool               `json:"headers_observed"`
	ResetSupported  bool               `json:"reset_supported"`
	FetchedAt       int64              `json:"fetched_at"`
}

type GrokQuotaResetResult struct {
	Supported bool   `json:"supported"`
	Code      string `json:"code"`
	Message   string `json:"message"`
}

type GrokBatchProbeItem struct {
	AccountID int64                 `json:"account_id"`
	OK        bool                  `json:"ok"`
	Class     string                `json:"class"` // ok | ok_partial | expired | transient
	Error     string                `json:"error,omitempty"`
	Result    *GrokQuotaProbeResult `json:"result,omitempty"`
}

type GrokBatchProbeResult struct {
	Total     int                  `json:"total"`
	OK        int                  `json:"ok"`
	Failed    int                  `json:"failed"`
	Expired   int                  `json:"expired"`
	Transient int                  `json:"transient"`
	Results   []GrokBatchProbeItem `json:"results"`
}

type GrokQuotaService struct {
	accountRepo   AccountRepository
	proxyRepo     ProxyRepository
	tokenProvider *GrokTokenProvider
	httpUpstream  HTTPUpstream
}

func NewGrokQuotaService(
	accountRepo AccountRepository,
	proxyRepo ProxyRepository,
	tokenProvider *GrokTokenProvider,
	httpUpstream HTTPUpstream,
) *GrokQuotaService {
	return &GrokQuotaService{
		accountRepo:   accountRepo,
		proxyRepo:     proxyRepo,
		tokenProvider: tokenProvider,
		httpUpstream:  httpUpstream,
	}
}

func (s *GrokQuotaService) ProbeUsage(ctx context.Context, accountID int64) (*GrokQuotaProbeResult, error) {
	account, err := s.loadGrokOAuthAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	return s.probeAccount(ctx, account, true)
}

// ProbeHeaders actively probes xAI rate-limit headers via GET /v1/models.
// Management API billing is optional and only attempted when credentials exist.
func (s *GrokQuotaService) ProbeHeaders(ctx context.Context, accountID int64) (*GrokQuotaProbeResult, error) {
	account, err := s.loadGrokOAuthAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	return s.probeAccount(ctx, account, false)
}

// BatchProbe probes multiple Grok OAuth accounts with bounded concurrency.
// Failure classification: expired (auth) vs transient (network/5xx/429).
func (s *GrokQuotaService) BatchProbe(ctx context.Context, accountIDs []int64, concurrency int) *GrokBatchProbeResult {
	out := &GrokBatchProbeResult{
		Total:   len(accountIDs),
		Results: make([]GrokBatchProbeItem, 0, len(accountIDs)),
	}
	if len(accountIDs) == 0 {
		return out
	}
	if concurrency <= 0 {
		concurrency = 5
	}
	if concurrency > 20 {
		concurrency = 20
	}

	type item struct {
		id  int64
		res GrokBatchProbeItem
	}
	ch := make(chan item, len(accountIDs))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for _, id := range accountIDs {
		wg.Add(1)
		go func(accountID int64) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			entry := GrokBatchProbeItem{AccountID: accountID}
			result, err := s.ProbeUsage(ctx, accountID)
			if err != nil {
				entry.OK = false
				entry.Error = err.Error()
				entry.Class = classifyGrokProbeError(err, nil)
			} else {
				entry.Result = result
				entry.Class = classifyGrokProbeResult(result)
				entry.OK = entry.Class == "ok" || entry.Class == "ok_partial"
				if !entry.OK && result != nil && result.ErrorMessage != "" {
					entry.Error = result.ErrorMessage
				}
			}
			ch <- item{id: accountID, res: entry}
		}(id)
	}
	wg.Wait()
	close(ch)
	for it := range ch {
		out.Results = append(out.Results, it.res)
		switch it.res.Class {
		case "ok", "ok_partial":
			out.OK++
		case "expired":
			out.Expired++
			out.Failed++
		default:
			out.Transient++
			out.Failed++
		}
	}
	return out
}

func (s *GrokQuotaService) probeAccount(ctx context.Context, account *Account, includeManagement bool) (*GrokQuotaProbeResult, error) {
	result := &GrokQuotaProbeResult{
		Source:         "header_probe",
		ResetSupported: false,
		FetchedAt:      time.Now().Unix(),
	}

	snapshot, statusCode, probeErr := s.probeRateLimitHeaders(ctx, account)
	result.StatusCode = statusCode
	if snapshot != nil {
		result.Snapshot = snapshot
		result.HeadersObserved = snapshot.HasObservedHeaders()
		s.persistQuotaSnapshot(ctx, account.ID, snapshot)
		s.applyProbeSideEffects(ctx, account, snapshot, statusCode)
		if snapshot.SubscriptionTier != "" || snapshot.EntitlementStatus != "" {
			s.persistTierHints(ctx, account.ID, snapshot.SubscriptionTier, snapshot.EntitlementStatus)
		}
	}
	if probeErr != nil && result.ErrorMessage == "" {
		result.ErrorMessage = probeErr.Error()
	}

	if !includeManagement {
		if result.HeadersObserved {
			result.Source = "header_probe"
		} else if result.ErrorMessage == "" {
			result.Source = "header_probe_no_headers"
			result.ErrorMessage = "未观测到 xAI rate-limit 响应头"
		}
		return result, nil
	}

	managementKey, teamID := grokManagementCredentials(account)
	if managementKey == "" || teamID == "" {
		if result.HeadersObserved {
			result.Source = "header_probe"
			// Keep header success; note management is optional.
			if result.ErrorMessage == "" {
				result.ErrorMessage = "未配置 Management API Key / Team ID，跳过官方 USD 用量"
			}
		} else {
			result.Source = "management_api_unconfigured"
			if result.ErrorMessage == "" {
				result.ErrorMessage = "未配置 xAI Management API Key / Team ID，且未观测到 rate-limit 响应头"
			}
		}
		return result, nil
	}

	usage, mgmtStatus, errMsg, err := s.fetchOfficialUsage(ctx, account, managementKey, teamID)
	if err != nil {
		// Header probe may still be useful; surface management error without failing whole call.
		if result.ErrorMessage == "" {
			result.ErrorMessage = err.Error()
		}
		if result.HeadersObserved {
			result.Source = "header_probe"
		} else {
			result.Source = "management_api_error"
		}
		return result, nil
	}
	if mgmtStatus > 0 {
		result.StatusCode = mgmtStatus
	}
	result.OfficialUsage = usage
	if usage != nil {
		_ = s.accountRepo.UpdateExtra(ctx, account.ID, map[string]any{grokOfficialUsageExtraKey: usage})
	}
	if errMsg != "" && result.ErrorMessage == "" {
		result.ErrorMessage = errMsg
	}
	switch {
	case result.HeadersObserved && usage != nil:
		result.Source = "combined"
	case usage != nil:
		result.Source = "management_api"
	case result.HeadersObserved:
		result.Source = "header_probe"
	default:
		result.Source = "management_api"
	}
	return result, nil
}

func (s *GrokQuotaService) probeRateLimitHeaders(ctx context.Context, account *Account) (*xai.QuotaSnapshot, int, error) {
	if s == nil || s.httpUpstream == nil {
		return nil, 0, infraerrors.New(http.StatusInternalServerError, "GROK_QUOTA_NOT_CONFIGURED", "grok quota service is not configured")
	}
	token, err := s.resolveAccessToken(ctx, account)
	if err != nil {
		return nil, 0, err
	}
	if strings.TrimSpace(token) == "" {
		return nil, 0, infraerrors.New(http.StatusBadRequest, "GROK_QUOTA_NO_TOKEN", "access_token is empty")
	}

	baseURL := strings.TrimRight(account.GetGrokBaseURL(), "/")
	if baseURL == "" {
		baseURL = xai.DefaultBaseURL
	}
	targetURL := baseURL + "/models"

	callCtx, cancel := context.WithTimeout(ctx, grokQuotaUpstreamTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(callCtx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, 0, infraerrors.Newf(http.StatusInternalServerError, "GROK_PROBE_REQUEST_BUILD_FAILED", "failed to build probe request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "sub2api-grok-quota-probe/1.0")

	resp, err := s.httpUpstream.Do(req, s.resolveProxyURL(ctx, account), account.ID, maxInt(account.Concurrency, 1))
	if err != nil {
		return nil, 0, infraerrors.Newf(http.StatusBadGateway, "GROK_PROBE_REQUEST_FAILED", "xAI models probe failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))

	snapshot := xai.ObserveQuotaHeaders(resp.Header, resp.StatusCode, "active_probe")
	if snapshot == nil {
		snapshot = &xai.QuotaSnapshot{
			StatusCode:        resp.StatusCode,
			ObservationSource: "active_probe",
			UpdatedAt:         time.Now().UTC().Format(time.RFC3339),
		}
	}
	now := time.Now().UTC().Format(time.RFC3339)
	snapshot.LastProbeAt = now
	if snapshot.HasObservedHeaders() {
		snapshot.LastHeadersSeenAt = now
		snapshot.HeadersObserved = true
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return snapshot, resp.StatusCode, infraerrors.Newf(resp.StatusCode, "GROK_PROBE_AUTH_FAILED", "xAI models probe returned %d", resp.StatusCode)
	}
	return snapshot, resp.StatusCode, nil
}

// ValidateAccessToken performs a minimal authenticated probe against xAI
// (GET {baseURL}/models) before AT-only account creation. Accepts 2xx and 429
// (auth succeeded; rate-limited). Any other status or transport error fails.
func (s *GrokQuotaService) ValidateAccessToken(ctx context.Context, accessToken, baseURL, proxyURL string) error {
	if s == nil || s.httpUpstream == nil {
		return fmt.Errorf("access_token upstream validation is not configured")
	}
	token := strings.TrimSpace(accessToken)
	if token == "" {
		return fmt.Errorf("access_token is empty")
	}
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		base = xai.DefaultBaseURL
	}
	targetURL := base + "/models"

	callCtx, cancel := context.WithTimeout(ctx, grokQuotaUpstreamTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(callCtx, http.MethodGet, targetURL, nil)
	if err != nil {
		return fmt.Errorf("failed to build access_token validation request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "sub2api-grok-at-validate/1.0")

	resp, err := s.httpUpstream.Do(req, strings.TrimSpace(proxyURL), 0, 1)
	if err != nil {
		return fmt.Errorf("xAI access_token validation failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	// 2xx: accepted. 429: credential is real but rate-limited.
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil
	}

	msg := strings.TrimSpace(extractUpstreamErrorMessage(body))
	if msg == "" {
		msg = strings.TrimSpace(string(body))
	}
	if msg == "" {
		msg = http.StatusText(resp.StatusCode)
	}
	// Keep message concise for import line results.
	if len(msg) > 300 {
		msg = msg[:300] + "..."
	}
	return fmt.Errorf("xAI rejected access_token (HTTP %d): %s", resp.StatusCode, msg)
}

func (s *GrokQuotaService) resolveAccessToken(ctx context.Context, account *Account) (string, error) {
	if account == nil {
		return "", infraerrors.New(http.StatusBadRequest, "GROK_QUOTA_INVALID_ACCOUNT", "account is nil")
	}
	if account.Type == AccountTypeAPIKey {
		return strings.TrimSpace(account.GetOpenAIApiKey()), nil
	}
	if s != nil && s.tokenProvider != nil {
		token, err := s.tokenProvider.GetAccessToken(ctx, account)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(token), nil
	}
	return strings.TrimSpace(account.GetGrokAccessToken()), nil
}

func (s *GrokQuotaService) persistQuotaSnapshot(ctx context.Context, accountID int64, snapshot *xai.QuotaSnapshot) {
	if s == nil || s.accountRepo == nil || accountID <= 0 || snapshot == nil {
		return
	}
	_ = s.accountRepo.UpdateExtra(ctx, accountID, map[string]any{grokQuotaSnapshotExtraKey: snapshot})
}

func (s *GrokQuotaService) persistTierHints(ctx context.Context, accountID int64, tier, entitlement string) {
	if s == nil || s.accountRepo == nil || accountID <= 0 {
		return
	}
	updates := map[string]any{}
	if strings.TrimSpace(tier) != "" {
		updates["subscription_tier"] = strings.TrimSpace(tier)
	}
	if strings.TrimSpace(entitlement) != "" {
		updates["entitlement_status"] = strings.TrimSpace(entitlement)
	}
	if len(updates) == 0 {
		return
	}
	_ = s.accountRepo.UpdateExtra(ctx, accountID, updates)
}

func (s *GrokQuotaService) applyProbeSideEffects(ctx context.Context, account *Account, snapshot *xai.QuotaSnapshot, statusCode int) {
	if s == nil || s.accountRepo == nil || account == nil {
		return
	}
	now := time.Now()
	switch statusCode {
	case http.StatusUnauthorized:
		_ = s.accountRepo.SetTempUnschedulable(ctx, account.ID, now.Add(10*time.Minute), "grok probe unauthorized")
	case http.StatusForbidden:
		_ = s.accountRepo.SetTempUnschedulable(ctx, account.ID, now.Add(30*time.Minute), "grok probe forbidden")
	case http.StatusTooManyRequests:
		cooldown := 2 * time.Minute
		if snapshot != nil && snapshot.RetryAfterSeconds != nil && *snapshot.RetryAfterSeconds > 0 {
			cooldown = time.Duration(*snapshot.RetryAfterSeconds) * time.Second
		}
		resetAt := now.Add(cooldown)
		_ = s.accountRepo.SetRateLimited(ctx, account.ID, resetAt)
		_ = s.accountRepo.SetTempUnschedulable(ctx, account.ID, resetAt, "grok probe rate limited")
	default:
		// Exhausted request window: cool until reset if remaining is zero.
		if snapshot != nil && snapshot.Requests != nil && snapshot.Requests.Remaining != nil && *snapshot.Requests.Remaining <= 0 {
			resetAt := now.Add(2 * time.Minute)
			if snapshot.Requests.ResetUnix != nil && *snapshot.Requests.ResetUnix > now.Unix() {
				resetAt = time.Unix(*snapshot.Requests.ResetUnix, 0)
			}
			_ = s.accountRepo.SetRateLimited(ctx, account.ID, resetAt)
		}
	}
}

func classifyGrokProbeResult(result *GrokQuotaProbeResult) string {
	if result == nil {
		return "transient"
	}
	switch result.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return "expired"
	case http.StatusTooManyRequests:
		return "transient"
	}
	if result.HeadersObserved || result.OfficialUsage != nil {
		if result.ErrorMessage != "" {
			return "ok_partial"
		}
		return "ok"
	}
	if strings.Contains(strings.ToLower(result.ErrorMessage), "unauthorized") ||
		strings.Contains(strings.ToLower(result.ErrorMessage), "forbidden") ||
		strings.Contains(strings.ToLower(result.ErrorMessage), "auth") {
		return "expired"
	}
	return "transient"
}

func classifyGrokProbeError(err error, result *GrokQuotaProbeResult) string {
	if result != nil {
		return classifyGrokProbeResult(result)
	}
	if err == nil {
		return "transient"
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "unauthorized") || strings.Contains(msg, "401") || strings.Contains(msg, "invalid_grant") || strings.Contains(msg, "no refresh token") {
		return "expired"
	}
	return "transient"
}

func (s *GrokQuotaService) fetchOfficialUsage(ctx context.Context, account *Account, managementKey, teamID string) (*xai.UsageSnapshot, int, string, error) {
	now := time.Now().UTC()
	usage, statusCode, errMsg, err := s.fetchOfficialUsageOnce(ctx, account, managementKey, teamID, now, true)
	if err != nil {
		return nil, statusCode, errMsg, err
	}
	if usage != nil || statusCode != http.StatusBadRequest {
		return usage, statusCode, errMsg, nil
	}
	fallbackUsage, fallbackStatus, fallbackMsg, fallbackErr := s.fetchOfficialUsageOnce(ctx, account, managementKey, teamID, now, false)
	if fallbackErr != nil {
		return nil, fallbackStatus, fallbackMsg, fallbackErr
	}
	if fallbackUsage != nil {
		return fallbackUsage, fallbackStatus, fallbackMsg, nil
	}
	return nil, statusCode, errMsg, nil
}

func (s *GrokQuotaService) fetchOfficialUsageOnce(ctx context.Context, account *Account, managementKey, teamID string, now time.Time, includeUsageMetric bool) (*xai.UsageSnapshot, int, string, error) {
	if s == nil || s.httpUpstream == nil {
		return nil, 0, "", infraerrors.New(http.StatusInternalServerError, "GROK_QUOTA_NOT_CONFIGURED", "grok quota service is not configured")
	}
	body, start, end, err := buildGrokOfficialUsageBody(now, includeUsageMetric)
	if err != nil {
		return nil, 0, "", infraerrors.Newf(http.StatusInternalServerError, "GROK_USAGE_BODY_ERROR", "failed to build usage body: %v", err)
	}
	targetURL, err := buildGrokManagementUsageURL(account, teamID)
	if err != nil {
		return nil, 0, "", infraerrors.Newf(http.StatusBadRequest, "GROK_USAGE_BASE_URL_INVALID", "invalid xAI management URL: %v", err)
	}

	callCtx, cancel := context.WithTimeout(ctx, grokQuotaUpstreamTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(callCtx, http.MethodPost, targetURL, bytes.NewReader(body))
	if err != nil {
		return nil, 0, "", infraerrors.Newf(http.StatusInternalServerError, "GROK_USAGE_REQUEST_BUILD_FAILED", "failed to build management request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+managementKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "sub2api-grok-usage/1.0")

	resp, err := s.httpUpstream.Do(req, s.resolveProxyURL(ctx, account), account.ID, maxInt(account.Concurrency, 1))
	if err != nil {
		return nil, 0, "", infraerrors.Newf(http.StatusBadGateway, "GROK_USAGE_REQUEST_FAILED", "xAI management usage request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyText := truncate(strings.TrimSpace(string(bodyBytes)), 240)
		slog.Warn("grok_official_usage_failed", "account_id", account.ID, "status", resp.StatusCode, "body", bodyText)
		return nil, resp.StatusCode, bodyText, nil
	}
	usage, err := parseGrokOfficialUsage(bodyBytes, start, end, includeUsageMetric)
	if err != nil {
		return nil, resp.StatusCode, "", infraerrors.Newf(http.StatusBadGateway, "GROK_USAGE_RESPONSE_INVALID", "invalid xAI management usage response: %v", err)
	}
	return usage, resp.StatusCode, "", nil
}

func (s *GrokQuotaService) ResetQuota(ctx context.Context, accountID int64) (*GrokQuotaResetResult, error) {
	if _, err := s.loadGrokOAuthAccount(ctx, accountID); err != nil {
		return nil, err
	}
	return nil, infraerrors.New(http.StatusNotImplemented, "GROK_QUOTA_RESET_UNSUPPORTED", "xAI does not expose a Grok subscription quota reset endpoint for OAuth accounts")
}

func (s *GrokQuotaService) resolveProxyURL(ctx context.Context, account *Account) string {
	if account == nil || account.ProxyID == nil {
		return ""
	}
	switch {
	case account.Proxy != nil:
		return account.Proxy.URL()
	case s != nil && s.proxyRepo != nil:
		if proxy, err := s.proxyRepo.GetByID(ctx, *account.ProxyID); err == nil && proxy != nil {
			return proxy.URL()
		}
	}
	return ""
}

func (s *GrokQuotaService) loadGrokOAuthAccount(ctx context.Context, accountID int64) (*Account, error) {
	if s == nil || s.accountRepo == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "GROK_QUOTA_NOT_CONFIGURED", "grok quota service is not configured")
	}
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, infraerrors.Newf(http.StatusNotFound, "GROK_QUOTA_ACCOUNT_NOT_FOUND", "account not found: %v", err)
	}
	if account == nil {
		return nil, infraerrors.New(http.StatusNotFound, "GROK_QUOTA_ACCOUNT_NOT_FOUND", "account not found")
	}
	if account.Platform != PlatformGrok {
		return nil, infraerrors.New(http.StatusBadRequest, "GROK_QUOTA_INVALID_PLATFORM", "account is not a Grok account")
	}
	if account.Type != AccountTypeOAuth && account.Type != AccountTypeAPIKey {
		return nil, infraerrors.New(http.StatusBadRequest, "GROK_QUOTA_INVALID_TYPE", "account is not a Grok OAuth/API key account")
	}
	return account, nil
}

func grokManagementCredentials(account *Account) (string, string) {
	if account == nil {
		return "", ""
	}
	managementKey := firstCredential(account, "management_api_key", "xai_management_api_key")
	teamID := firstCredential(account, "team_id", "xai_team_id")
	return managementKey, teamID
}

func firstCredential(account *Account, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(account.GetCredential(key)); value != "" {
			return value
		}
	}
	return ""
}

func buildGrokOfficialUsageBody(now time.Time, includeUsageMetric bool) ([]byte, time.Time, time.Time, error) {
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	values := []map[string]string{{
		"name":        grokOfficialUsageDefaultValue,
		"aggregation": "AGGREGATION_SUM",
	}}
	if includeUsageMetric {
		values = append(values, map[string]string{
			"name":        grokOfficialUsageTokenValue,
			"aggregation": "AGGREGATION_SUM",
		})
	}
	payload := map[string]any{
		"analyticsRequest": map[string]any{
			"timeRange": map[string]any{
				"startTime": start.Format("2006-01-02 15:04:05"),
				"endTime":   now.Format("2006-01-02 15:04:05"),
				"timezone":  grokOfficialUsageDefaultZone,
			},
			"timeUnit": "TIME_UNIT_DAY",
			"values":   values,
			"groupBy":  []string{"description"},
			"filters":  []any{},
		},
	}
	body, err := json.Marshal(payload)
	return body, start, now, err
}

func buildGrokManagementUsageURL(account *Account, teamID string) (string, error) {
	base := strings.TrimSpace(account.GetCredential("management_base_url"))
	if base == "" {
		base = grokQuotaManagementBaseURL
	}
	parsed, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	parsed.RawQuery = ""
	baseURL := strings.TrimRight(parsed.String(), "/")
	return baseURL + "/v1/billing/teams/" + url.PathEscape(teamID) + "/usage", nil
}

type grokOfficialUsageResponse struct {
	TimeSeries   []grokOfficialUsageSeries `json:"timeSeries"`
	LimitReached bool                      `json:"limitReached"`
}

type grokOfficialUsageSeries struct {
	DataPoints []grokOfficialUsagePoint `json:"dataPoints"`
}

type grokOfficialUsagePoint struct {
	Values []float64 `json:"values"`
}

func parseGrokOfficialUsage(data []byte, start, end time.Time, includeUsageMetric bool) (*xai.UsageSnapshot, error) {
	var response grokOfficialUsageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, err
	}
	var usd float64
	var usage float64
	hasUsageValue := false
	for _, series := range response.TimeSeries {
		for _, point := range series.DataPoints {
			if len(point.Values) > 0 {
				usd += point.Values[0]
			}
			if includeUsageMetric && len(point.Values) > 1 {
				usage += point.Values[1]
				hasUsageValue = true
			}
		}
	}
	now := time.Now().UTC().Format(time.RFC3339)
	snapshot := &xai.UsageSnapshot{
		Source:       "management_api",
		ValueName:    grokOfficialUsageDefaultValue,
		USD:          usd,
		StartTime:    start.Format(time.RFC3339),
		EndTime:      end.Format(time.RFC3339),
		Timezone:     grokOfficialUsageDefaultZone,
		LimitReached: response.LimitReached,
		UpdatedAt:    now,
	}
	if includeUsageMetric && hasUsageValue {
		snapshot.UsageName = grokOfficialUsageTokenValue
		snapshot.Usage = &usage
	}
	return snapshot, nil
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
