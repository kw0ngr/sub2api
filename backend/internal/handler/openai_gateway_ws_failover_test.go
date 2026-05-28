package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type openAIWSFailoverAccountRepo struct {
	service.AccountRepository
	mu             sync.Mutex
	accounts       []service.Account
	rateLimitedIDs []int64
}

func (r *openAIWSFailoverAccountRepo) ListSchedulableByPlatform(_ context.Context, platform string) ([]service.Account, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	accounts := make([]service.Account, 0, len(r.accounts))
	for _, account := range r.accounts {
		if account.Platform == platform && account.IsSchedulable() {
			accounts = append(accounts, account)
		}
	}
	return accounts, nil
}

func (r *openAIWSFailoverAccountRepo) GetByID(_ context.Context, id int64) (*service.Account, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, account := range r.accounts {
		if account.ID == id {
			copyAccount := account
			return &copyAccount, nil
		}
	}
	return nil, nil
}

func (r *openAIWSFailoverAccountRepo) SetRateLimited(_ context.Context, id int64, resetAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rateLimitedIDs = append(r.rateLimitedIDs, id)
	for index := range r.accounts {
		if r.accounts[index].ID == id {
			reset := resetAt
			r.accounts[index].RateLimitResetAt = &reset
		}
	}
	return nil
}

func TestOpenAIResponsesWebSocket_FailoverOnInitialUsageLimitEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)

	firstHit := make(chan struct{}, 1)
	secondHit := make(chan struct{}, 1)
	firstUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		conn, err := coderws.Accept(w, req, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.CloseNow() }()
		readCtx, cancelRead := context.WithTimeout(req.Context(), 3*time.Second)
		_, _, err = conn.Read(readCtx)
		cancelRead()
		if err != nil {
			return
		}
		firstHit <- struct{}{}
		writeCtx, cancelWrite := context.WithTimeout(req.Context(), 3*time.Second)
		_ = conn.Write(writeCtx, coderws.MessageText, []byte(`{"type":"error","error":{"code":"rate_limit_exceeded","type":"usage_limit_reached","message":"usage limited","resets_at":4102444800}}`))
		cancelWrite()
	}))
	defer firstUpstream.Close()

	secondUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		conn, err := coderws.Accept(w, req, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.CloseNow() }()
		readCtx, cancelRead := context.WithTimeout(req.Context(), 3*time.Second)
		_, _, err = conn.Read(readCtx)
		cancelRead()
		if err != nil {
			return
		}
		secondHit <- struct{}{}
		writeCtx, cancelWrite := context.WithTimeout(req.Context(), 3*time.Second)
		_ = conn.Write(writeCtx, coderws.MessageText, []byte(`{"type":"response.completed","response":{"id":"resp_ws_failover_ok","model":"gpt-5.1","usage":{"input_tokens":1,"output_tokens":1}}}`))
		cancelWrite()
	}))
	defer secondUpstream.Close()

	groupID := int64(4202)
	repo := &openAIWSFailoverAccountRepo{accounts: []service.Account{
		{
			ID: 9902, Name: "rate-limited", Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
			Status: service.StatusActive, Schedulable: true, Concurrency: 1, Priority: 1,
			Credentials: map[string]any{"api_key": "sk-first", "base_url": firstUpstream.URL},
			Extra:       map[string]any{"openai_apikey_responses_websockets_v2_mode": service.OpenAIWSIngressModePassthrough},
		},
		{
			ID: 9903, Name: "healthy", Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
			Status: service.StatusActive, Schedulable: true, Concurrency: 1, Priority: 2,
			Credentials: map[string]any{"api_key": "sk-second", "base_url": secondUpstream.URL},
			Extra:       map[string]any{"openai_apikey_responses_websockets_v2_mode": service.OpenAIWSIngressModePassthrough},
		},
	}}
	cfg := &config.Config{}
	cfg.RunMode = config.RunModeSimple
	cfg.Default.RateMultiplier = 1
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Gateway.MaxAccountSwitches = 3
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.ModeRouterV2Enabled = true
	cfg.Gateway.OpenAIWS.IngressModeDefault = service.OpenAIWSIngressModeCtxPool
	cfg.Gateway.OpenAIWS.DialTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.WriteTimeoutSeconds = 3

	rateLimitService := service.NewRateLimitService(repo, nil, cfg, nil, nil)
	billingCacheService := service.NewBillingCacheService(nil, nil, nil, nil, cfg)
	t.Cleanup(billingCacheService.Stop)
	gatewayService := service.NewOpenAIGatewayService(
		repo, nil, nil, nil, nil, nil, nil, cfg, nil, nil,
		service.NewBillingService(cfg, nil), rateLimitService, billingCacheService,
		nil, &service.DeferredService{}, nil, nil, nil, nil, nil,
	)
	cache := &concurrencyCacheMock{
		acquireUserSlotFn:    func(context.Context, int64, int, string) (bool, error) { return true, nil },
		acquireAccountSlotFn: func(context.Context, int64, int, string) (bool, error) { return true, nil },
	}
	handler := &OpenAIGatewayHandler{
		gatewayService:      gatewayService,
		billingCacheService: billingCacheService,
		apiKeyService:       &service.APIKeyService{},
		concurrencyHelper:   NewConcurrencyHelper(service.NewConcurrencyService(cache), SSEPingFormatNone, time.Second),
		maxAccountSwitches:  3,
	}
	apiKey := &service.APIKey{
		ID: 1802, GroupID: &groupID,
		User:  &service.User{ID: 1702, Status: service.StatusActive},
		Group: &service.Group{ID: groupID, Platform: service.PlatformOpenAI, Status: service.StatusActive},
	}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyAPIKey), apiKey)
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: apiKey.User.ID, Concurrency: 1})
		c.Next()
	})
	router.GET("/openai/v1/responses", handler.ResponsesWebSocket)
	server := httptest.NewServer(router)
	defer server.Close()

	dialCtx, cancelDial := context.WithTimeout(context.Background(), 3*time.Second)
	clientConn, _, err := coderws.Dial(dialCtx, "ws"+strings.TrimPrefix(server.URL, "http")+"/openai/v1/responses", nil)
	cancelDial()
	require.NoError(t, err)
	defer func() { _ = clientConn.CloseNow() }()

	writeCtx, cancelWrite := context.WithTimeout(context.Background(), 3*time.Second)
	err = clientConn.Write(writeCtx, coderws.MessageText, []byte(`{"type":"response.create","model":"gpt-5.1","stream":false}`))
	cancelWrite()
	require.NoError(t, err)

	readCtx, cancelRead := context.WithTimeout(context.Background(), 5*time.Second)
	_, event, err := clientConn.Read(readCtx)
	cancelRead()
	require.NoError(t, err)
	require.Equal(t, "response.completed", gjson.GetBytes(event, "type").String())
	require.Equal(t, "resp_ws_failover_ok", gjson.GetBytes(event, "response.id").String())
	require.Eventually(t, func() bool { return len(firstHit) == 1 && len(secondHit) == 1 }, 3*time.Second, 20*time.Millisecond)
	require.Equal(t, []int64{9902}, repo.rateLimitedIDs)
}
