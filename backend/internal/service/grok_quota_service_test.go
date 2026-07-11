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
	account      *Account
	extraUpdates map[string]any
}

func (r *grokQuotaAccountRepoStub) GetByID(_ context.Context, id int64) (*Account, error) {
	if r.account == nil || r.account.ID != id {
		return nil, ErrAccountNotFound
	}
	return r.account, nil
}

func (r *grokQuotaAccountRepoStub) UpdateExtra(_ context.Context, _ int64, updates map[string]any) error {
	r.extraUpdates = updates
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
	// Given
	account := &Account{
		ID:          1894,
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Credentials: map[string]any{"access_token": "token"},
		Concurrency: 1,
	}
	repo := &grokQuotaAccountRepoStub{account: account}
	upstream := &grokQuotaHTTPUpstreamStub{}
	svc := NewGrokQuotaService(repo, nil, nil, upstream)

	// When
	result, err := svc.ProbeUsage(context.Background(), account.ID)

	// Then
	require.NoError(t, err)
	require.Equal(t, "management_api_unconfigured", result.Source)
	require.Contains(t, result.ErrorMessage, "Management API")
	require.Equal(t, 0, upstream.calls)
}

func TestGrokQuotaService_ProbeUsage_fetchesOfficialManagementUsage(t *testing.T) {
	// Given
	account := &Account{
		ID:       1894,
		Platform: PlatformGrok,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"management_api_key": "mgmt-key",
			"team_id":            "team-123",
		},
		Concurrency: 1,
	}
	repo := &grokQuotaAccountRepoStub{account: account}
	upstream := &grokQuotaHTTPUpstreamStub{
		response: &http.Response{
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
	}
	svc := NewGrokQuotaService(repo, nil, nil, upstream)

	// When
	result, err := svc.ProbeUsage(context.Background(), account.ID)

	// Then
	require.NoError(t, err)
	require.Equal(t, "management_api", result.Source)
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
}

func TestGrokQuotaService_ProbeUsage_fallsBackToUsdOnlyWhenUsageMetricUnsupported(t *testing.T) {
	// Given
	account := &Account{
		ID:       1894,
		Platform: PlatformGrok,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"management_api_key": "mgmt-key",
			"team_id":            "team-123",
		},
		Concurrency: 1,
	}
	repo := &grokQuotaAccountRepoStub{account: account}
	upstream := &grokQuotaHTTPUpstreamStub{
		responses: []*http.Response{
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
	require.Equal(t, 2, upstream.calls)
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
