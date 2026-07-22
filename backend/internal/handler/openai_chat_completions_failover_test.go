package handler

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type openAIChatCyberUpstream struct {
	mu     sync.Mutex
	hitIDs []int64
}

func (u *openAIChatCyberUpstream) Do(_ *http.Request, _ string, accountID int64, _ int) (*http.Response, error) {
	u.mu.Lock()
	u.hitIDs = append(u.hitIDs, accountID)
	u.mu.Unlock()
	return &http.Response{
		StatusCode: http.StatusBadGateway,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(
			`{"error":{"type":"content_policy","code":"cyber_policy","message":"This content was flagged for possible cybersecurity risk."}}`,
		)),
	}, nil
}

func (u *openAIChatCyberUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, concurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, concurrency)
}

func (u *openAIChatCyberUpstream) hits() []int64 {
	u.mu.Lock()
	defer u.mu.Unlock()
	return append([]int64(nil), u.hitIDs...)
}

func TestOpenAIChatCompletionsCyberPolicyDoesNotSweepAccountPool(t *testing.T) {
	gin.SetMode(gin.TestMode)

	groupID := int64(4210)
	accounts := make([]service.Account, 0, 8)
	for i := int64(1); i <= 8; i++ {
		accounts = append(accounts, service.Account{
			ID:          10_000 + i,
			Name:        "cyber-test",
			Platform:    service.PlatformOpenAI,
			Type:        service.AccountTypeAPIKey,
			Status:      service.StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Priority:    int(i),
			Credentials: map[string]any{"api_key": "sk-test", "base_url": "https://upstream.invalid"},
		})
	}
	repo := &openAIWSFailoverAccountRepo{accounts: accounts}
	upstream := &openAIChatCyberUpstream{}
	cfg := &config.Config{}
	cfg.RunMode = config.RunModeSimple
	cfg.Default.RateMultiplier = 1
	cfg.Gateway.MaxAccountSwitches = 50

	rateLimitService := service.NewRateLimitService(repo, nil, cfg, nil, nil)
	billingCacheService := service.NewBillingCacheService(nil, nil, nil, nil, cfg)
	t.Cleanup(billingCacheService.Stop)
	gatewayService := service.NewOpenAIGatewayService(
		repo, nil, nil, nil, nil, nil, nil, cfg, nil, nil,
		service.NewBillingService(cfg, nil), rateLimitService, billingCacheService,
		upstream, &service.DeferredService{}, nil, nil, nil, nil, nil, nil,
	)
	cache := &concurrencyCacheMock{
		acquireUserSlotFn:    func(context.Context, int64, int, string) (bool, error) { return true, nil },
		acquireAccountSlotFn: func(context.Context, int64, int, string) (bool, error) { return true, nil },
	}
	h := NewOpenAIGatewayHandler(
		gatewayService,
		service.NewConcurrencyService(cache),
		billingCacheService,
		&service.APIKeyService{},
		nil,
		nil,
		cfg,
	)
	apiKey := &service.APIKey{
		ID:      1810,
		GroupID: &groupID,
		User:    &service.User{ID: 1710, Status: service.StatusActive},
		Group:   &service.Group{ID: groupID, Platform: service.PlatformOpenAI, Status: service.StatusActive},
	}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyAPIKey), apiKey)
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: apiKey.User.ID, Concurrency: 1})
		c.Next()
	})
	router.POST("/v1/chat/completions", h.ChatCompletions)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost,
		"/v1/chat/completions",
		strings.NewReader(`{"model":"gpt-5.6-sol","messages":[{"role":"user","content":"hi"}],"stream":true}`),
	)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)

	hits := upstream.hits()
	require.Len(t, hits, openAICyberSafetyRetryMaxSwitches+1, "initial attempt plus three bounded account switches")
	require.Len(t, mapFromAccountIDs(hits), len(hits), "each retry must use a different account")
	require.Less(t, len(hits), len(accounts), "the remaining healthy pool must not be swept")
	require.Equal(t, http.StatusBadGateway, recorder.Code)
}

func mapFromAccountIDs(ids []int64) map[int64]struct{} {
	result := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		result[id] = struct{}{}
	}
	return result
}
