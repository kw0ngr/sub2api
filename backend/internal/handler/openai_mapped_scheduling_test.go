package handler

import (
	"context"
	"fmt"
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

func TestOpenAIChannelForwardModelForScheduler(t *testing.T) {
	repo := &task8MappedSchedulingAccountRepo{accounts: []service.Account{{
		ID: 8008, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
		Status: service.StatusActive, Schedulable: true, Concurrency: 1,
		Credentials: map[string]any{"model_mapping": map[string]any{"gpt-6-astra": "gpt-6-astra-upstream"}},
	}}}
	cfg := &config.Config{RunMode: config.RunModeSimple}
	svc := service.NewOpenAIGatewayService(repo, nil, nil, nil, nil, nil, nil, cfg, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	selection, _, err := svc.SelectAccountWithSchedulerForPlatform(context.Background(), service.PlatformOpenAI, nil, "", "task8", "gpt-6-astra", nil, service.OpenAIUpstreamTransportAny)
	require.NoError(t, err)
	require.Equal(t, int64(8008), selection.Account.ID)
	upstream, matched := selection.Account.ResolveMappedModel("gpt-6-astra")
	require.True(t, matched)
	require.Equal(t, "gpt-6-astra-upstream", upstream)
}

func TestOpenAIResponsesWebSocket_ChannelMappedTargetSelectsAccountWithoutRequestedAlias(t *testing.T) {
	got := runOpenAIResponsesWebSocketUsageLogCase(t, openAIResponsesWSUsageLogCase{
		firstPayload: `{"type":"response.create","model":"public-astra","stream":false}`,
		channelMapping: map[string]string{
			"public-astra": "gpt-6-astra",
		},
		accountModelMapping: map[string]any{
			"gpt-6-astra": "gpt-6-astra",
		},
	})

	require.Equal(t, "gpt-6-astra", gjson.GetBytes(got.upstreamFirstPayload, "model").String())
	require.Equal(t, "public-astra", gjson.GetBytes(got.clientEvent, "response.model").String())
	require.Equal(t, "public-astra", got.log.RequestedModel)
	require.NotNil(t, got.log.UpstreamModel)
	require.Equal(t, "gpt-6-astra", *got.log.UpstreamModel)
	require.NotNil(t, got.log.ModelMappingChain)
	require.Equal(t, "public-astra→gpt-6-astra", *got.log.ModelMappingChain)
}

func TestOpenAIModelMappingUnsupportedMappedTargetReturnsModelNotFoundNot429(t *testing.T) {
	diagnoser := &fakeModelAvailabilityDiagnoser{
		resp: service.ModelAvailabilityDiagnosis{HasAccountsInPool: true, HasModelSupport: false},
	}
	classification := classifyNoAccountError(context.Background(), noAccountDiagnosisRequest{
		Diagnoser:    diagnoser,
		APIKey:       &service.APIKey{GroupID: groupIDPtr(8)},
		RoutingModel: "gpt-unsupported-mapped-target",
		DisplayModel: "public-astra",
		Platform:     service.PlatformOpenAI,
	})

	require.Equal(t, http.StatusNotFound, classification.Status)
	require.NotEqual(t, http.StatusTooManyRequests, classification.Status)
	require.Equal(t, "model_not_found", classification.ErrType)
	require.True(t, classification.ModelNotFound)
	require.Len(t, diagnoser.calls, 1)
	require.Equal(t, "gpt-unsupported-mapped-target", diagnoser.calls[0].model)
}

type task8MappedSchedulingAccountRepo struct {
	service.AccountRepository
	accounts []service.Account
}

func (r *task8MappedSchedulingAccountRepo) ListSchedulableByPlatform(_ context.Context, platform string) ([]service.Account, error) {
	accounts := make([]service.Account, 0, len(r.accounts))
	for _, account := range r.accounts {
		if account.Platform == platform && account.IsSchedulable() {
			accounts = append(accounts, account)
		}
	}
	return accounts, nil
}

func (r *task8MappedSchedulingAccountRepo) ListSchedulableByGroupIDAndPlatform(_ context.Context, _ int64, platform string) ([]service.Account, error) {
	return r.ListSchedulableByPlatform(context.Background(), platform)
}

func (r *task8MappedSchedulingAccountRepo) GetByID(_ context.Context, id int64) (*service.Account, error) {
	for _, account := range r.accounts {
		if account.ID == id {
			copyAccount := account
			return &copyAccount, nil
		}
	}
	return nil, nil
}

type openAIResponsesWSUsageLogCase struct {
	firstPayload        string
	channelMapping      map[string]string
	accountModelMapping map[string]any
}

type openAIResponsesWSUsageLogResult struct {
	log                  *service.UsageLog
	upstreamFirstPayload []byte
	clientEvent          []byte
}

type task8WSUsageLogRepo struct {
	service.UsageLogRepository
	created chan *service.UsageLog
}

func (r *task8WSUsageLogRepo) Create(_ context.Context, log *service.UsageLog) (bool, error) {
	r.created <- log
	return true, nil
}

type task8WSChannelRepo struct {
	service.ChannelRepository
	mu       sync.Mutex
	channels []service.Channel
	groups   map[int64]string
}

func (r *task8WSChannelRepo) ListAll(_ context.Context) ([]service.Channel, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]service.Channel(nil), r.channels...), nil
}

func (r *task8WSChannelRepo) GetGroupPlatforms(_ context.Context, groupIDs []int64) (map[int64]string, error) {
	out := make(map[int64]string, len(groupIDs))
	for _, groupID := range groupIDs {
		out[groupID] = r.groups[groupID]
	}
	return out, nil
}

func runOpenAIResponsesWebSocketUsageLogCase(t *testing.T, tc openAIResponsesWSUsageLogCase) openAIResponsesWSUsageLogResult {
	t.Helper()
	gin.SetMode(gin.TestMode)

	upstreamPayload := make(chan []byte, 1)
	upstreamError := make(chan error, 1)
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := coderws.Accept(w, r, &coderws.AcceptOptions{CompressionMode: coderws.CompressionContextTakeover})
		if err != nil {
			upstreamError <- err
			return
		}
		defer func() { _ = conn.CloseNow() }()
		readCtx, cancelRead := context.WithTimeout(r.Context(), 3*time.Second)
		_, payload, err := conn.Read(readCtx)
		cancelRead()
		if err != nil {
			upstreamError <- err
			return
		}
		upstreamPayload <- payload
		response := fmt.Sprintf(`{"type":"response.completed","response":{"id":"resp_task8","model":%q,"usage":{"input_tokens":2,"output_tokens":1}}}`, gjson.GetBytes(payload, "model").String())
		writeCtx, cancelWrite := context.WithTimeout(r.Context(), 3*time.Second)
		err = conn.Write(writeCtx, coderws.MessageText, []byte(response))
		cancelWrite()
		upstreamError <- err
	}))
	defer upstreamServer.Close()

	groupID := int64(4201)
	account := service.Account{
		ID: 9901, Name: "task8-ws", Platform: service.PlatformOpenAI,
		Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, Concurrency: 1,
		Credentials: map[string]any{
			"api_key":       "test",
			"base_url":      upstreamServer.URL,
			"model_mapping": tc.accountModelMapping,
		},
		Extra: map[string]any{
			"openai_apikey_responses_websockets_v2_enabled": true,
			"openai_apikey_responses_websockets_v2_mode":    service.OpenAIWSIngressModePassthrough,
		},
	}
	accountRepo := &task8MappedSchedulingAccountRepo{accounts: []service.Account{account}}
	usageRepo := &task8WSUsageLogRepo{created: make(chan *service.UsageLog, 1)}
	channelService := service.NewChannelService(&task8WSChannelRepo{
		channels: []service.Channel{{
			ID: 7701, Name: "task8", Status: service.StatusActive, GroupIDs: []int64{groupID},
			ModelMapping: map[string]map[string]string{service.PlatformOpenAI: tc.channelMapping},
		}},
		groups: map[int64]string{groupID: service.PlatformOpenAI},
	}, nil, nil, nil)

	cfg := &config.Config{RunMode: config.RunModeSimple}
	cfg.Default.RateMultiplier = 1
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.ModeRouterV2Enabled = true
	cfg.Gateway.OpenAIWS.DialTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.WriteTimeoutSeconds = 3
	billingCache := service.NewBillingCacheService(nil, nil, nil, nil, cfg)
	t.Cleanup(billingCache.Stop)
	gatewayService := service.NewOpenAIGatewayService(
		accountRepo, usageRepo, nil, nil, nil, nil, nil, cfg, nil, nil,
		service.NewBillingService(cfg, nil), nil, billingCache, nil, &service.DeferredService{},
		nil, nil, nil, channelService, nil, nil,
	)
	cache := &concurrencyCacheMock{
		acquireUserSlotFn:    func(context.Context, int64, int, string) (bool, error) { return true, nil },
		acquireAccountSlotFn: func(context.Context, int64, int, string) (bool, error) { return true, nil },
	}
	handler := &OpenAIGatewayHandler{
		gatewayService: gatewayService, billingCacheService: billingCache, apiKeyService: &service.APIKeyService{},
		concurrencyHelper: NewConcurrencyHelper(service.NewConcurrencyService(cache), SSEPingFormatNone, time.Second), cfg: cfg,
	}

	apiKey := &service.APIKey{ID: 1801, GroupID: &groupID, User: &service.User{ID: 1701, Status: service.StatusActive}}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyAPIKey), apiKey)
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: apiKey.User.ID, Concurrency: 1})
		c.Next()
	})
	router.GET("/openai/v1/responses", handler.ResponsesWebSocket)
	handlerServer := httptest.NewServer(router)
	defer handlerServer.Close()

	dialCtx, cancelDial := context.WithTimeout(context.Background(), 3*time.Second)
	client, _, err := coderws.Dial(dialCtx, "ws"+strings.TrimPrefix(handlerServer.URL, "http")+"/openai/v1/responses", nil)
	cancelDial()
	require.NoError(t, err)
	defer func() { _ = client.CloseNow() }()
	writeCtx, cancelWrite := context.WithTimeout(context.Background(), 3*time.Second)
	err = client.Write(writeCtx, coderws.MessageText, []byte(tc.firstPayload))
	cancelWrite()
	require.NoError(t, err)
	readCtx, cancelRead := context.WithTimeout(context.Background(), 3*time.Second)
	_, clientEvent, err := client.Read(readCtx)
	cancelRead()
	require.NoError(t, err)
	require.Equal(t, "response.completed", gjson.GetBytes(clientEvent, "type").String())
	_ = client.Close(coderws.StatusNormalClosure, "done")

	result := openAIResponsesWSUsageLogResult{clientEvent: clientEvent}
	select {
	case result.log = <-usageRepo.created:
	case <-time.After(3 * time.Second):
		t.Fatal("waiting for task8 websocket usage log")
	}
	select {
	case result.upstreamFirstPayload = <-upstreamPayload:
	case <-time.After(3 * time.Second):
		t.Fatal("waiting for task8 upstream websocket payload")
	}
	select {
	case err = <-upstreamError:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("waiting for task8 upstream websocket completion")
	}
	return result
}
