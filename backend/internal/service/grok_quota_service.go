package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
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

	result := &GrokQuotaProbeResult{
		Source:         "management_api",
		ResetSupported: false,
		FetchedAt:      time.Now().Unix(),
	}
	managementKey, teamID := grokManagementCredentials(account)
	if managementKey == "" || teamID == "" {
		result.Source = "management_api_unconfigured"
		result.ErrorMessage = "未配置 xAI Management API Key / Team ID，无法查询官方真实用量。"
		return result, nil
	}

	usage, statusCode, errMsg, err := s.fetchOfficialUsage(ctx, account, managementKey, teamID)
	if err != nil {
		return nil, err
	}
	result.StatusCode = statusCode
	result.OfficialUsage = usage
	if usage != nil {
		_ = s.accountRepo.UpdateExtra(ctx, account.ID, map[string]any{grokOfficialUsageExtraKey: usage})
	}
	if errMsg != "" {
		result.ErrorMessage = errMsg
	}
	return result, nil
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
	if account.Type != AccountTypeOAuth {
		return nil, infraerrors.New(http.StatusBadRequest, "GROK_QUOTA_INVALID_TYPE", "account is not an OAuth account")
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
