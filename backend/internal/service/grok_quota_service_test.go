package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
)

type grokQuotaAccountRepoStub struct {
	AccountRepository
	account           *Account
	extraUpdates      map[string]any
	rateLimitedUntil  *time.Time
	tempUnschedUntil  *time.Time
	tempUnschedReason string
	clearErrorCalls   int
	setErrorCalls     int
	lastErrorMsg      string
	clearTempCalls    int
	clearRateCalls    int
	setSchedulableTo  *bool
	setSchedulableN   int
}

func (r *grokQuotaAccountRepoStub) GetByID(_ context.Context, id int64) (*Account, error) {
	if r.account == nil || r.account.ID != id {
		return nil, ErrAccountNotFound
	}
	return r.account, nil
}

func (r *grokQuotaAccountRepoStub) UpdateExtra(_ context.Context, _ int64, updates map[string]any) error {
	if r.extraUpdates == nil {
		r.extraUpdates = map[string]any{}
	}
	for k, v := range updates {
		r.extraUpdates[k] = v
	}
	return nil
}

func (r *grokQuotaAccountRepoStub) SetRateLimited(_ context.Context, _ int64, resetAt time.Time) error {
	r.rateLimitedUntil = &resetAt
	return nil
}

func (r *grokQuotaAccountRepoStub) SetTempUnschedulable(_ context.Context, _ int64, until time.Time, reason string) error {
	r.tempUnschedUntil = &until
	r.tempUnschedReason = reason
	return nil
}

func (r *grokQuotaAccountRepoStub) SetError(_ context.Context, id int64, msg string) error {
	r.setErrorCalls++
	r.lastErrorMsg = msg
	if r.account != nil && r.account.ID == id {
		r.account.Status = StatusError
		r.account.ErrorMessage = msg
	}
	return nil
}

func (r *grokQuotaAccountRepoStub) ClearError(_ context.Context, id int64) error {
	r.clearErrorCalls++
	if r.account != nil && r.account.ID == id {
		r.account.Status = StatusActive
		r.account.ErrorMessage = ""
	}
	return nil
}

func (r *grokQuotaAccountRepoStub) ClearTempUnschedulable(_ context.Context, id int64) error {
	r.clearTempCalls++
	r.tempUnschedUntil = nil
	r.tempUnschedReason = ""
	if r.account != nil && r.account.ID == id {
		r.account.TempUnschedulableUntil = nil
		r.account.TempUnschedulableReason = ""
	}
	return nil
}

func (r *grokQuotaAccountRepoStub) ClearRateLimit(_ context.Context, _ int64) error {
	r.clearRateCalls++
	r.rateLimitedUntil = nil
	return nil
}

func (r *grokQuotaAccountRepoStub) SetSchedulable(_ context.Context, id int64, schedulable bool) error {
	r.setSchedulableN++
	r.setSchedulableTo = &schedulable
	if r.account != nil && r.account.ID == id {
		r.account.Schedulable = schedulable
	}
	return nil
}

type grokQuotaHTTPUpstreamStub struct {
	requestBody string
	requestURL  string
	authHeader  string
	calls       int
	response    *http.Response
	responses   []*http.Response
	err         error
}

func (u *grokQuotaHTTPUpstreamStub) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	u.calls++
	u.requestURL = req.URL.String()
	u.authHeader = req.Header.Get("Authorization")
	if req.Body != nil {
		body, _ := io.ReadAll(req.Body)
		u.requestBody = string(body)
	}
	if len(u.responses) > 0 {
		resp := u.responses[0]
		u.responses = u.responses[1:]
		return resp, u.err
	}
	return u.response, u.err
}

func (u *grokQuotaHTTPUpstreamStub) DoWithTLS(req *http.Request, proxyURL string, accountID int64, concurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, concurrency)
}

func TestGrokQuotaService_ProbeUsage_requiresManagementCredentials(t *testing.T) {
	// Given: no management credentials, but header probe still runs.
	account := &Account{
		ID:          1894,
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Credentials: map[string]any{"access_token": "token"},
		Concurrency: 1,
	}
	repo := &grokQuotaAccountRepoStub{account: account}
	headers := http.Header{}
	headers.Set("x-ratelimit-limit-requests", "60")
	headers.Set("x-ratelimit-remaining-requests", "42")
	headers.Set("x-ratelimit-reset-requests", "120")
	headers.Set("xai-subscription-tier", "super")
	upstream := &grokQuotaHTTPUpstreamStub{
		response: &http.Response{
			StatusCode: http.StatusOK,
			Header:     headers,
			Body:       io.NopCloser(strings.NewReader(`{"data":[]}`)),
		},
	}
	svc := NewGrokQuotaService(repo, nil, nil, upstream)

	// When
	result, err := svc.ProbeUsage(context.Background(), account.ID)

	// Then
	require.NoError(t, err)
	require.Equal(t, "header_probe", result.Source)
	require.True(t, result.HeadersObserved)
	require.NotNil(t, result.Snapshot)
	require.NotNil(t, result.Snapshot.Requests)
	require.Equal(t, int64(42), *result.Snapshot.Requests.Remaining)
	require.Equal(t, 1, upstream.calls)
	require.Contains(t, repo.extraUpdates, "grok_usage_snapshot")
	require.Equal(t, "super", repo.extraUpdates["subscription_tier"])
}

func TestGrokQuotaService_ProbeUsage_fetchesOfficialManagementUsage(t *testing.T) {
	// Given
	account := &Account{
		ID:       1894,
		Platform: PlatformGrok,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":       "token",
			"management_api_key": "mgmt-key",
			"team_id":            "team-123",
		},
		Concurrency: 1,
	}
	repo := &grokQuotaAccountRepoStub{account: account}
	modelHeaders := http.Header{}
	modelHeaders.Set("x-ratelimit-limit-requests", "60")
	modelHeaders.Set("x-ratelimit-remaining-requests", "50")
	upstream := &grokQuotaHTTPUpstreamStub{
		responses: []*http.Response{
			{
				StatusCode: http.StatusOK,
				Header:     modelHeaders,
				Body:       io.NopCloser(strings.NewReader(`{"data":[]}`)),
			},
			{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body: io.NopCloser(strings.NewReader(`{
"timeSeries":[
{"dataPoints":[{"values":[1.25,1000]},{"values":[2.5,2500]}]},
{"dataPoints":[{"values":[0.75,750]}]}
],
"limitReached":false
}`)),
			},
		},
	}
	svc := NewGrokQuotaService(repo, nil, nil, upstream)

	// When
	result, err := svc.ProbeUsage(context.Background(), account.ID)

	// Then
	require.NoError(t, err)
	require.Equal(t, "combined", result.Source)
	require.True(t, result.HeadersObserved)
	require.Equal(t, http.StatusOK, result.StatusCode)
	require.NotNil(t, result.OfficialUsage)
	require.InDelta(t, 4.5, result.OfficialUsage.USD, 0.0001)
	require.NotNil(t, result.OfficialUsage.Usage)
	require.InDelta(t, 4250, *result.OfficialUsage.Usage, 0.0001)
	require.Equal(t, "management_api", result.OfficialUsage.Source)
	require.Equal(t, "Bearer mgmt-key", upstream.authHeader)
	require.Equal(t, "https://management-api.x.ai/v1/billing/teams/team-123/usage", upstream.requestURL)
	require.Contains(t, upstream.requestBody, `"analyticsRequest"`)
	require.Contains(t, upstream.requestBody, `"usd"`)
	require.Contains(t, upstream.requestBody, `"usage"`)
	require.Contains(t, repo.extraUpdates, grokOfficialUsageExtraKey)
	require.Contains(t, repo.extraUpdates, "grok_usage_snapshot")
}

func TestGrokQuotaService_ProbeUsage_fallsBackToUsdOnlyWhenUsageMetricUnsupported(t *testing.T) {
	// Given
	account := &Account{
		ID:       1894,
		Platform: PlatformGrok,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":       "token",
			"management_api_key": "mgmt-key",
			"team_id":            "team-123",
		},
		Concurrency: 1,
	}
	repo := &grokQuotaAccountRepoStub{account: account}
	// Provide rate-limit headers on /models so chat fallback is skipped and the
	// remaining stub responses map to management usage (usd+usage -> usd-only).
	modelHeaders := http.Header{}
	modelHeaders.Set("x-ratelimit-limit-requests", "60")
	modelHeaders.Set("x-ratelimit-remaining-requests", "50")
	upstream := &grokQuotaHTTPUpstreamStub{
		responses: []*http.Response{
			{
				StatusCode: http.StatusOK,
				Header:     modelHeaders,
				Body:       io.NopCloser(strings.NewReader(`{"data":[]}`)),
			},
			{
				StatusCode: http.StatusBadRequest,
				Body:       io.NopCloser(strings.NewReader(`{"error":"unknown field usage"}`)),
			},
			{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(`{
					"timeSeries":[{"dataPoints":[{"values":[1.25]}]}],
					"limitReached":false
				}`)),
			},
		},
	}
	svc := NewGrokQuotaService(repo, nil, nil, upstream)

	// When
	result, err := svc.ProbeUsage(context.Background(), account.ID)

	// Then
	require.NoError(t, err)
	require.Equal(t, 3, upstream.calls)
	require.NotNil(t, result.OfficialUsage)
	require.InDelta(t, 1.25, result.OfficialUsage.USD, 0.0001)
	require.Nil(t, result.OfficialUsage.Usage)
	require.NotContains(t, upstream.requestBody, `"usage"`)
}

func TestParseGrokOfficialUsage_sumsUsdValues(t *testing.T) {
	// Given
	start := time.Date(2026, 7, 11, 0, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	payload := []byte(`{"timeSeries":[{"dataPoints":[{"values":[0.1,10]},{"values":[0.2,20]}]}],"limitReached":true}`)

	// When
	usage, err := parseGrokOfficialUsage(payload, start, end, true)

	// Then
	require.NoError(t, err)
	require.InDelta(t, 0.3, usage.USD, 0.0001)
	require.NotNil(t, usage.Usage)
	require.InDelta(t, 30, *usage.Usage, 0.0001)
	require.True(t, usage.LimitReached)
	require.Equal(t, start.Format(time.RFC3339), usage.StartTime)
	require.Equal(t, end.Format(time.RFC3339), usage.EndTime)
}

func TestBuildGrokManagementUsageURL_allowsCustomBaseURL(t *testing.T) {
	// Given
	account := &Account{Credentials: map[string]any{"management_base_url": "https://proxy.example.com/root/"}}

	// When
	actual, err := buildGrokManagementUsageURL(account, "team/with/slash")

	// Then
	require.NoError(t, err)
	require.Equal(t, "https://proxy.example.com/root/v1/billing/teams/team%2Fwith%2Fslash/usage", actual)
}

func TestGrokQuotaFetcher_BuildUsageInfo_readsOfficialUsage(t *testing.T) {
	// Given
	account := &Account{
		Extra: map[string]any{
			grokOfficialUsageExtraKey: map[string]any{
				"source":     "management_api",
				"usd":        7.25,
				"updated_at": "2026-07-11T00:00:00Z",
			},
		},
	}

	// When
	usage := NewGrokQuotaFetcher().BuildUsageInfo(account)

	// Then
	require.NotNil(t, usage.GrokOfficialUsage)
	require.InDelta(t, 7.25, usage.GrokOfficialUsage.USD, 0.0001)
	require.Empty(t, usage.ErrorCode)
}

func TestBuildGrokOfficialUsageBody_usesUtcDayWindow(t *testing.T) {
	// Given
	now := time.Date(2026, 7, 11, 5, 30, 0, 0, time.UTC)

	// When
	body, start, end, err := buildGrokOfficialUsageBody(now, true)

	// Then
	require.NoError(t, err)
	require.Equal(t, time.Date(2026, 7, 11, 0, 0, 0, 0, time.UTC), start)
	require.Equal(t, now, end)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(body, &payload))
	require.Contains(t, string(body), `"TIME_UNIT_DAY"`)
	require.Contains(t, string(body), `"Etc/GMT"`)
	require.Contains(t, string(body), `"usage"`)
}

func TestGrokQuotaService_ProbeHeaders_DisablesCallsOnUnauthorized(t *testing.T) {
	account := &Account{
		ID:          42,
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Credentials: map[string]any{"access_token": "bad"},
		Concurrency: 1,
	}
	repo := &grokQuotaAccountRepoStub{account: account}
	upstream := &grokQuotaHTTPUpstreamStub{
		response: &http.Response{
			StatusCode: http.StatusUnauthorized,
			Header:     http.Header{},
			Body:       io.NopCloser(strings.NewReader(`{"error":"unauthorized"}`)),
		},
	}
	svc := NewGrokQuotaService(repo, nil, nil, upstream)
	result, err := svc.ProbeHeaders(context.Background(), account.ID)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, result.StatusCode)
	require.Nil(t, repo.tempUnschedUntil)
	require.Empty(t, repo.tempUnschedReason)
	require.Equal(t, 1, repo.setErrorCalls)
	require.Contains(t, repo.lastErrorMsg, "unauthorized")
	require.Equal(t, 1, repo.setSchedulableN)
	require.NotNil(t, repo.setSchedulableTo)
	require.False(t, *repo.setSchedulableTo)
}

func TestGrokQuotaService_ProbeHeaders_DoesNotReenableDisabledAccountOnUnauthorized(t *testing.T) {
	account := &Account{
		ID:          43,
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Status:      StatusDisabled,
		Schedulable: false,
		Credentials: map[string]any{"access_token": "bad"},
		Concurrency: 1,
	}
	repo := &grokQuotaAccountRepoStub{account: account}
	upstream := &grokQuotaHTTPUpstreamStub{
		response: &http.Response{
			StatusCode: http.StatusUnauthorized,
			Header:     http.Header{},
			Body:       io.NopCloser(strings.NewReader(`{"error":"unauthorized"}`)),
		},
	}
	svc := NewGrokQuotaService(repo, nil, nil, upstream)
	_, err := svc.ProbeHeaders(context.Background(), account.ID)
	require.NoError(t, err)
	require.Zero(t, repo.setErrorCalls)
	require.Zero(t, repo.setSchedulableN)
}

func TestClassifyGrokProbeResult(t *testing.T) {
	require.Equal(t, "expired", classifyGrokProbeResult(&GrokQuotaProbeResult{StatusCode: 401}))
	require.Equal(t, "transient", classifyGrokProbeResult(&GrokQuotaProbeResult{StatusCode: 429}))
	require.Equal(t, "ok", classifyGrokProbeResult(&GrokQuotaProbeResult{HeadersObserved: true}))
	require.Equal(t, "ok_partial", classifyGrokProbeResult(&GrokQuotaProbeResult{HeadersObserved: true, ErrorMessage: "no mgmt"}))
}

func TestGrokQuotaService_ValidateAccessToken(t *testing.T) {
	t.Parallel()

	t.Run("accepts 200", func(t *testing.T) {
		upstream := &grokQuotaHTTPUpstreamStub{
			response: &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{},
				Body:       io.NopCloser(strings.NewReader(`{"data":[]}`)),
			},
		}
		svc := NewGrokQuotaService(nil, nil, nil, upstream)
		require.NoError(t, svc.ValidateAccessToken(context.Background(), "tok", "", ""))
		require.Equal(t, "https://api.x.ai/v1/models", upstream.requestURL)
		require.Equal(t, "Bearer tok", upstream.authHeader)
	})

	t.Run("accepts 429 as authenticated", func(t *testing.T) {
		upstream := &grokQuotaHTTPUpstreamStub{
			response: &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Header:     http.Header{},
				Body:       io.NopCloser(strings.NewReader(`{"error":"rate limited"}`)),
			},
		}
		svc := NewGrokQuotaService(nil, nil, nil, upstream)
		require.NoError(t, svc.ValidateAccessToken(context.Background(), "tok", "", ""))
	})

	t.Run("rejects 401", func(t *testing.T) {
		upstream := &grokQuotaHTTPUpstreamStub{
			response: &http.Response{
				StatusCode: http.StatusUnauthorized,
				Header:     http.Header{},
				Body:       io.NopCloser(strings.NewReader(`{"error":"Incorrect API key provided"}`)),
			},
		}
		svc := NewGrokQuotaService(nil, nil, nil, upstream)
		err := svc.ValidateAccessToken(context.Background(), "bad", "", "")
		require.Error(t, err)
		require.Contains(t, err.Error(), "rejected access_token")
		require.Contains(t, err.Error(), "Incorrect API key")
	})

	t.Run("not configured", func(t *testing.T) {
		svc := NewGrokQuotaService(nil, nil, nil, nil)
		err := svc.ValidateAccessToken(context.Background(), "tok", "", "")
		require.Error(t, err)
		require.Contains(t, err.Error(), "not configured")
	})
}

func TestGrokQuotaService_ProbeUsage_recoversHealthyAccount(t *testing.T) {
	// Given: previously failed OAuth account that is actually healthy now.
	until := time.Now().Add(20 * time.Minute)
	account := &Account{
		ID:                      1943,
		Platform:                PlatformGrok,
		Type:                    AccountTypeOAuth,
		Status:                  StatusError,
		Schedulable:             false,
		ErrorMessage:            "Access forbidden (403): account may be suspended or lack permissions",
		TempUnschedulableUntil:  &until,
		TempUnschedulableReason: "grok probe forbidden",
		Credentials: map[string]any{
			"access_token": "token",
		},
		Concurrency: 1,
	}
	repo := &grokQuotaAccountRepoStub{account: account}
	headers := http.Header{}
	headers.Set("x-ratelimit-limit-requests", "60")
	headers.Set("x-ratelimit-remaining-requests", "42")
	headers.Set("x-ratelimit-reset-requests", "120")
	upstream := &grokQuotaHTTPUpstreamStub{
		response: &http.Response{
			StatusCode: http.StatusOK,
			Header:     headers,
			Body:       io.NopCloser(strings.NewReader(`{"data":[]}`)),
		},
	}
	svc := NewGrokQuotaService(repo, nil, nil, upstream)

	// When
	result, err := svc.ProbeUsage(context.Background(), account.ID)

	// Then
	require.NoError(t, err)
	require.True(t, result.HeadersObserved)
	require.Equal(t, 1, repo.clearErrorCalls)
	require.Equal(t, 1, repo.clearTempCalls)
	require.Equal(t, 1, repo.clearRateCalls)
	require.Equal(t, 1, repo.setSchedulableN)
	require.NotNil(t, repo.setSchedulableTo)
	require.True(t, *repo.setSchedulableTo)
	require.Equal(t, StatusActive, account.Status)
	require.True(t, account.Schedulable)
	require.Empty(t, account.ErrorMessage)
	require.Nil(t, account.TempUnschedulableUntil)
}

func TestGrokQuotaService_ProbeUsage_doesNotRecoverDisabledAccount(t *testing.T) {
	account := &Account{
		ID:           2001,
		Platform:     PlatformGrok,
		Type:         AccountTypeOAuth,
		Status:       StatusDisabled,
		Schedulable:  false,
		ErrorMessage: "manually disabled",
		Credentials:  map[string]any{"access_token": "token"},
		Concurrency:  1,
	}
	repo := &grokQuotaAccountRepoStub{account: account}
	headers := http.Header{}
	headers.Set("x-ratelimit-limit-requests", "60")
	headers.Set("x-ratelimit-remaining-requests", "10")
	upstream := &grokQuotaHTTPUpstreamStub{
		response: &http.Response{
			StatusCode: http.StatusOK,
			Header:     headers,
			Body:       io.NopCloser(strings.NewReader(`{"data":[]}`)),
		},
	}
	svc := NewGrokQuotaService(repo, nil, nil, upstream)

	_, err := svc.ProbeUsage(context.Background(), account.ID)
	require.NoError(t, err)
	require.Equal(t, 0, repo.clearErrorCalls)
	require.Equal(t, 0, repo.setSchedulableN)
	require.Equal(t, StatusDisabled, account.Status)
	require.False(t, account.Schedulable)
}

func TestIsRecoverableGrokProbeError(t *testing.T) {
	require.True(t, isRecoverableGrokProbeError(""))
	require.True(t, isRecoverableGrokProbeError("Access forbidden (403): account may be suspended or lack permissions"))
	require.True(t, isRecoverableGrokProbeError("grok probe forbidden"))
	require.True(t, isRecoverableGrokProbeError("token_refresh_failed: no refresh token"))
	require.False(t, isRecoverableGrokProbeError("billing hard fail permanently"))
}
