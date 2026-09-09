package admin

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type accountHandlerUpstreamModelsHTTP struct {
	lastReq *http.Request
	resp    *http.Response
	err     error
}

func (u *accountHandlerUpstreamModelsHTTP) Do(req *http.Request, proxyURL string, accountID int64, accountConcurrency int) (*http.Response, error) {
	return u.DoWithTLS(req, proxyURL, accountID, accountConcurrency, nil)
}

func (u *accountHandlerUpstreamModelsHTTP) DoWithTLS(req *http.Request, _ string, _ int64, _ int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	u.lastReq = req
	if u.err != nil {
		return nil, u.err
	}
	return u.resp, nil
}

func accountHandlerUpstreamModelsConfig() *config.Config {
	return &config.Config{
		Security: config.SecurityConfig{
			URLAllowlist: config.URLAllowlistConfig{Enabled: false},
		},
	}
}

func TestAccountHandlerSyncUpstreamModelsPreviewUsesProvidedCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := &accountHandlerUpstreamModelsHTTP{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"data":[{"id":"gpt-5"},{"id":"o3"}]}`)),
	}}
	handler := &AccountHandler{
		accountTestService: service.NewAccountTestService(
			nil,
			nil,
			nil,
			nil,
			nil,
			upstream,
			accountHandlerUpstreamModelsConfig(),
			nil,
			nil,
		),
	}

	router := gin.New()
	router.POST("/admin/accounts/models/sync-upstream-preview", handler.SyncUpstreamModelsPreview)

	req := httptest.NewRequest(http.MethodPost, "/admin/accounts/models/sync-upstream-preview", strings.NewReader(`{
		"platform":"openai",
		"type":"apikey",
		"base_url":"https://openai.example.com/v1",
		"api_key":"sk-preview"
	}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"models":["gpt-5","o3"]`)
	require.NotNil(t, upstream.lastReq)
	require.Equal(t, "https://openai.example.com/v1/models", upstream.lastReq.URL.String())
	require.Equal(t, "Bearer sk-preview", upstream.lastReq.Header.Get("Authorization"))
}

func TestAccountHandlerSyncUpstreamModelsPreviewRejectsMissingAPIKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := &AccountHandler{accountTestService: &service.AccountTestService{}}
	router := gin.New()
	router.POST("/admin/accounts/models/sync-upstream-preview", handler.SyncUpstreamModelsPreview)

	req := httptest.NewRequest(http.MethodPost, "/admin/accounts/models/sync-upstream-preview", strings.NewReader(`{
		"platform":"openai",
		"type":"apikey",
		"base_url":"https://openai.example.com/v1"
	}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

type task6AdminModelMetadataRepoStub struct {
	service.AccountRepository
	updates map[string]any
}

func (r *task6AdminModelMetadataRepoStub) UpdateExtra(_ context.Context, _ int64, updates map[string]any) error {
	r.updates = updates
	return nil
}

func TestAccountHandlerSyncUpstreamModelsPersistsModelMetadataSnapshot(t *testing.T) {
	// Given
	gin.SetMode(gin.TestMode)
	repo := &task6AdminModelMetadataRepoStub{}
	upstream := &accountHandlerUpstreamModelsHTTP{resp: &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"data":[{"id":"gpt-6-astra"}]}`))}}
	adminSvc := &availableModelsAdminService{stubAdminService: newStubAdminService(), account: service.Account{ID: 708, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Credentials: map[string]any{"api_key": "openai-key", "base_url": "https://api.openai.example/v1"}}}
	handler := NewAccountHandler(adminSvc, nil, nil, nil, nil, nil, nil, nil, service.NewAccountTestService(repo, nil, nil, nil, nil, upstream, accountHandlerUpstreamModelsConfig(), nil, nil), nil, nil, nil, nil, nil)
	router := gin.New()
	router.POST("/admin/accounts/:id/models/sync-upstream", handler.SyncUpstreamModels)

	// When
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/accounts/708/models/sync-upstream", nil)
	router.ServeHTTP(w, req)

	// Then
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"models":["gpt-6-astra"]`)
	require.Contains(t, repo.updates, "upstream_model_metadata")
}
