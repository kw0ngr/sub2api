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
	response    *http.Response
	err         error
}

func (u *grokQuotaHTTPUpstreamStub) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	if req.Body != nil {
		body, _ := io.ReadAll(req.Body)
		u.requestBody = string(body)
	}
	return u.response, u.err
}

func (u *grokQuotaHTTPUpstreamStub) DoWithTLS(req *http.Request, proxyURL string, accountID int64, concurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, concurrency)
}

func TestBuildGrokQuotaProbeBody_usesDefaultModelMapping(t *testing.T) {
	// Given
	account := &Account{
		Credentials: map[string]any{
			"model_mapping": map[string]any{
				grokQuotaDefaultModel: "grok-4.5",
			},
		},
	}

	// When
	body, err := buildGrokQuotaProbeBody(account)

	// Then
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(body, &payload))
	require.Equal(t, "grok-4.5", payload["model"])
}

func TestGrokQuotaService_ProbeUsage_returnsSnapshotWhenUpstreamRejectsProbe(t *testing.T) {
	// Given
	expiresAt := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	account := &Account{
		ID:       1894,
		Platform: PlatformGrok,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token": "token",
			"expires_at":   expiresAt,
			"base_url":     "https://api.x.ai/v1",
		},
		Concurrency: 1,
	}
	repo := &grokQuotaAccountRepoStub{account: account}
	upstream := &grokQuotaHTTPUpstreamStub{
		response: &http.Response{
			StatusCode: http.StatusForbidden,
			Header:     http.Header{"X-Ratelimit-Limit-Requests": []string{"100"}},
			Body:       io.NopCloser(strings.NewReader(`{"code":"permission-denied","error":"credits exhausted"}`)),
		},
	}
	svc := NewGrokQuotaService(repo, nil, NewGrokTokenProvider(repo, nil), upstream)

	// When
	result, err := svc.ProbeUsage(context.Background(), account.ID)

	// Then
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, http.StatusForbidden, result.StatusCode)
	require.Contains(t, result.ErrorMessage, "credits exhausted")
	require.NotNil(t, result.Snapshot)
	require.True(t, result.HeadersObserved)
	require.Contains(t, upstream.requestBody, `"`+grokQuotaDefaultModel+`"`)
	require.Contains(t, repo.extraUpdates, grokQuotaSnapshotExtraKey)
}
