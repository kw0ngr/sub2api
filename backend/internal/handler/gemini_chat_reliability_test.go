package handler

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type geminiChatRegressionUpstream struct {
	service.HTTPUpstream
	do func(*http.Request) (*http.Response, error)
}

func (upstream *geminiChatRegressionUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	return upstream.do(req)
}

func geminiRegressionRouter(t *testing.T, do func(*http.Request) (*http.Response, error), platforms ...string) *gin.Engine {
	t.Helper()
	platform := service.PlatformGemini
	baseURL := "https://generativelanguage.googleapis.com"
	if len(platforms) > 0 {
		platform = platforms[0]
		if platform == service.PlatformGrok {
			baseURL = "https://api.x.ai/v1"
		}
	}
	groupID := int64(17)
	repo := &openAIWSFailoverAccountRepo{}
	for i := int64(1); i <= 6; i++ {
		repo.accounts = append(repo.accounts, service.Account{ID: i, Platform: platform,
			Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, Concurrency: 1,
			Credentials: map[string]any{"api_key": "test", "base_url": baseURL}})
	}
	cfg := &config.Config{}
	cfg.RunMode = config.RunModeSimple
	cfg.Default.RateMultiplier = 1
	cfg.Gateway.MaxAccountSwitches = 50
	billing := service.NewBillingCacheService(nil, nil, nil, nil, cfg)
	t.Cleanup(billing.Stop)
	gateway := service.NewOpenAIGatewayService(repo, nil, nil, nil, nil, nil, nil, cfg, nil, nil,
		service.NewBillingService(cfg, nil), nil, billing, &geminiChatRegressionUpstream{do: do},
		&service.DeferredService{}, nil, nil, nil, nil, nil, nil)
	cache := &concurrencyCacheMock{
		acquireUserSlotFn:    func(context.Context, int64, int, string) (bool, error) { return true, nil },
		acquireAccountSlotFn: func(context.Context, int64, int, string) (bool, error) { return true, nil },
	}
	h := NewOpenAIGatewayHandler(gateway, service.NewConcurrencyService(cache), billing, &service.APIKeyService{}, nil, nil, cfg)
	key := &service.APIKey{ID: 33, GroupID: &groupID, User: &service.User{ID: 1, Status: service.StatusActive},
		Group: &service.Group{ID: groupID, Platform: platform, Status: service.StatusActive}}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyAPIKey), key)
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 1, Concurrency: 1})
		c.Next()
	})
	router.POST("/v1/chat/completions", h.ChatCompletions)
	router.POST("/v1/responses", h.Responses)
	return router
}

func TestGeminiExpiredToolHistoryReturns400WithoutNetworkOrFailover(t *testing.T) {
	calls := 0
	router := geminiRegressionRouter(t, func(req *http.Request) (*http.Response, error) {
		calls++
		return nil, context.Canceled
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(
		`{"model":"gemini-3.8-flash","messages":[{"role":"user","content":"hi"},{"role":"assistant","tool_calls":[{"id":"call_old","type":"function","function":{"name":"lookup","arguments":"{}"}}]},{"role":"tool","tool_call_id":"call_old","content":"OK"}]}`))
	router.ServeHTTP(rec, req)
	require.Equal(t, 400, rec.Code)
	require.Contains(t, rec.Body.String(), "gemini_tool_signature_unavailable")
	require.Zero(t, calls)
}

func TestGeminiCanceledClientIsNotReportedAs502(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	router := geminiRegressionRouter(t, func(req *http.Request) (*http.Response, error) {
		cancel()
		<-req.Context().Done()
		return nil, req.Context().Err()
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions",
		strings.NewReader(`{"model":"gemini-3.8-flash","messages":[{"role":"user","content":"hi"}]}`)).WithContext(ctx)
	router.ServeHTTP(rec, req)
	require.Equal(t, 499, rec.Code)
	require.Empty(t, rec.Body.String())
}

func TestGemini503DoesNotSweepFiftyAccounts(t *testing.T) {
	calls := 0
	router := geminiRegressionRouter(t, func(req *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 503, Header: http.Header{"Content-Type": {"application/json"}},
			Body: io.NopCloser(strings.NewReader(`{"error":{"message":"This model is currently experiencing high demand."}}`))}, nil
	})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/chat/completions",
		strings.NewReader(`{"model":"gemini-3.8-flash","messages":[{"role":"user","content":"hi"}]}`)))
	require.Equal(t, 3, calls, "one initial account plus at most two switches")
	require.GreaterOrEqual(t, rec.Code, 400)
}

func TestGeminiGatewayTimeoutDoesNotFabricateUpstreamHTTPStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	(&OpenAIGatewayHandler{}).handleFailoverExhausted(c, &service.UpstreamFailoverError{LocalTimeout: true, StatusCode: 504}, false)
	require.Equal(t, 504, rec.Code)
	_, exists := c.Get(service.OpsUpstreamStatusCodeKey)
	require.False(t, exists)
	require.Equal(t, "upstream_timeout", normalizeOpsErrorType("upstream_timeout", ""))
	require.Equal(t, "network", classifyOpsPhase("upstream_timeout", "", ""))
}

func TestGeminiCancellationIsRecordedAsClientNotProviderFailure(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set(service.OpsClientCanceledKey, true)
	entry := &service.OpsInsertErrorLogInput{StatusCode: 499, ErrorOwner: "provider", ErrorSource: "upstream_http", IsRetryable: true}
	applyOpsClientCancellation(c, entry)
	require.Equal(t, "client", entry.ErrorOwner)
	require.Equal(t, "client_request", entry.ErrorSource)
	require.Equal(t, "client_canceled", entry.ErrorType)
	require.False(t, entry.IsRetryable)
	require.Nil(t, entry.UpstreamStatusCode)
}
