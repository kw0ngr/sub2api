//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const task15ResponseModelSource = "response_model"

func TestOpenAIForward_ObservesResponseModelStreamAndNonStream(t *testing.T) {
	streamBody := strings.Join([]string{
		`data: {"type":"response.created","response":{"id":"resp_stream","model":"gpt-5.4-nano","status":"in_progress"}}`,
		`data: {"type":"response.output_text.delta","delta":"hi"}`,
		`data: {"type":"response.completed","response":{"id":"resp_stream","model":"gpt-5.4-mini","status":"completed","usage":{"input_tokens":2,"output_tokens":1}}}`,
		`data: [DONE]`,
	}, "\n")

	streamResult := task15ForwardOpenAIPassthrough(t, true, "text/event-stream", streamBody)
	require.Equal(t, "gpt-5.4-mini", streamResult.UpstreamResponseModel)
	require.True(t, streamResult.UpstreamResponseModelConflict)

	nonStreamResult := task15ForwardOpenAIPassthrough(t, false, "application/json", `{"id":"resp_json","object":"response","model":"gpt5.4nano","status":"completed","output":[],"usage":{"input_tokens":3,"output_tokens":2,"total_tokens":5}}`)
	require.Equal(t, "gpt5.4nano", nonStreamResult.UpstreamResponseModel)
	require.False(t, nonStreamResult.UpstreamResponseModelConflict)
}

func TestTask15ObservedUpstreamServiceTierFromNonStreamResponse(t *testing.T) {
	// Given: upstream returns a real non-stream Responses JSON with service_tier and request omitted service_tier.
	result := task15ForwardOpenAIPassthrough(t, false, "application/json", `{"id":"resp_tier","object":"response","model":"gpt-5.6-sol","service_tier":"flex","status":"completed","output":[],"usage":{"input_tokens":3,"output_tokens":2,"total_tokens":5}}`)

	// Then: the observed upstream service_tier feeds the forward result for billing/account stats.
	require.NotNil(t, result.ServiceTier, "observed upstream service_tier must feed billing/account stats even when request omitted service_tier")
	require.Equal(t, "flex", *result.ServiceTier)

	// When: the real Forward result is recorded for usage/account stats.
	groupID := int64(10)
	usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
	svc := newOpenAIRecordUsageServiceForTest(usageRepo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
	svc.channelService = newTestChannelServiceForStats(t, &Channel{ID: 1, Status: StatusActive}, groupID, "openai")
	svc.billingService = newTestBillingServiceWithPrices(map[string]*ModelPricing{
		"gpt-5.6-sol": {InputPricePerToken: 0.001, OutputPricePerToken: 0.002},
	})
	err := svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{
		Result:  result,
		APIKey:  &APIKey{ID: 11, Quota: 100, GroupID: &groupID},
		User:    &User{ID: 21},
		Account: &Account{ID: 62},
	})

	// Then: the observed tier is persisted and account stats use flex pricing.
	require.NoError(t, err)
	require.NotNil(t, usageRepo.lastLog)
	require.Equal(t, "flex", *usageRepo.lastLog.ServiceTier)
	require.NotNil(t, usageRepo.lastLog.AccountStatsCost)
	require.InDelta(t, 0.0035, *usageRepo.lastLog.AccountStatsCost, 1e-12)
}

func TestTask15ObservedUpstreamServiceTierFromStreamingTerminalResponse(t *testing.T) {
	// Given: upstream SSE reports an early tier, then a terminal tier.
	streamBody := strings.Join([]string{
		`data: {"type":"response.created","response":{"id":"resp_stream_tier","model":"gpt-5.6-sol","service_tier":"priority","status":"in_progress"}}`,
		`data: {"type":"response.in_progress","response":{"id":"resp_stream_tier","model":"gpt-5.6-sol","service_tier":"priority","status":"in_progress"}}`,
		`data: {"type":"response.completed","response":{"id":"resp_stream_tier","model":"gpt-5.6-sol","service_tier":"flex","status":"completed","usage":{"input_tokens":2,"output_tokens":1}}}`,
		`data: [DONE]`,
	}, "\n")

	// When.
	result := task15ForwardOpenAIPassthrough(t, true, "text/event-stream", streamBody)

	// Then: terminal observed tier wins over earlier created/in-progress tiers.
	require.NotNil(t, result.ServiceTier)
	require.Equal(t, "flex", *result.ServiceTier)
}

func TestTask15MalformedObservedUpstreamServiceTierDoesNotCreateBillingState(t *testing.T) {
	// Given: upstream emits an unknown tier and request omitted service_tier.
	result := task15ForwardOpenAIPassthrough(t, false, "application/json", `{"id":"resp_bad_tier","object":"response","model":"gpt-5.6-sol","service_tier":"warp","status":"completed","output":[],"usage":{"input_tokens":3,"output_tokens":2,"total_tokens":5}}`)

	// Then: unknown observed tier is ignored instead of persisted into billing state.
	require.Nil(t, result.ServiceTier)
}

func TestOpenAIGatewayServiceRecordUsage_ResponseModelBillingGuards(t *testing.T) {
	usageTokens := UsageTokens{InputTokens: 20, OutputTokens: 10}

	tests := []struct {
		name          string
		baselineModel string
		responseModel string
		conflict      bool
		wantModel     string
	}{
		{name: "exact cheaper response adopted", baselineModel: "gpt-5.5", responseModel: "gpt-5.4-nano", wantModel: "gpt-5.4-nano"},
		{name: "alias cheaper response adopted", baselineModel: "gpt-5.5", responseModel: "GPT5.4NANO", wantModel: "gpt-5.4-nano"},
		{name: "ambiguous response rejected", baselineModel: "gpt-5.5", responseModel: "gpt-5.4-nano", conflict: true, wantModel: "gpt-5.5"},
		{name: "pricier response rejected", baselineModel: "gpt-5.4-nano", responseModel: "gpt-5.5", wantModel: "gpt-5.4-nano"},
		{name: "missing response pricing falls back", baselineModel: "gpt-5.5", responseModel: "zz-unpriced-response-model", wantModel: "gpt-5.5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
			userRepo := &openAIRecordUsageUserRepoStub{}
			svc := newOpenAIRecordUsageServiceForTest(usageRepo, userRepo, &openAIRecordUsageSubRepoStub{}, nil)
			wantCost, err := svc.billingService.CalculateCost(tt.wantModel, usageTokens, 1.1)
			require.NoError(t, err)
			result := &OpenAIForwardResult{
				RequestID:     "task15_response_model_" + strings.ReplaceAll(tt.name, " ", "_"),
				Model:         "gpt-5.6-sol",
				BillingModel:  tt.baselineModel,
				UpstreamModel: tt.baselineModel,
				Usage:         OpenAIUsage{InputTokens: 20, OutputTokens: 10},
				Duration:      time.Second,
			}
			result.UpstreamResponseModel = tt.responseModel
			result.UpstreamResponseModelConflict = tt.conflict

			err = svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{
				Result:  result,
				APIKey:  &APIKey{ID: 10, Quota: 100},
				User:    &User{ID: 20},
				Account: &Account{ID: 30},
				ChannelUsageFields: ChannelUsageFields{
					OriginalModel:      "gpt-5.6-sol",
					ChannelMappedModel: tt.baselineModel,
					BillingModelSource: task15ResponseModelSource,
				},
			})

			require.NoError(t, err)
			require.NotNil(t, usageRepo.lastLog)
			require.InDelta(t, wantCost.ActualCost, usageRepo.lastLog.ActualCost, 1e-12)
			require.InDelta(t, wantCost.ActualCost, userRepo.lastAmount, 1e-12)
			if tt.name == "exact cheaper response adopted" {
				require.Equal(t, "gpt-5.6-sol", usageRepo.lastLog.Model)
				require.Equal(t, "gpt-5.6-sol", usageRepo.lastLog.RequestedModel)
				require.Equal(t, tt.baselineModel, *usageRepo.lastLog.UpstreamModel)
				require.Equal(t, tt.responseModel, *usageRepo.lastLog.UpstreamResponseModel)
				require.True(t, *usageRepo.lastLog.UpstreamModelMismatch)
			}
		})
	}
}

func TestAccountStatsCost_UsesObservedServiceTierOnlyForModelFileFallback(t *testing.T) {
	tokens := UsageTokens{InputTokens: 100, OutputTokens: 50, CacheCreationTokens: 20, CacheReadTokens: 10}
	channel := &Channel{ID: 1, Status: StatusActive, ApplyPricingToAccountStats: false}
	cs := newTestChannelServiceForStats(t, channel, 10, "openai")
	bs := newTestBillingServiceWithPrices(map[string]*ModelPricing{
		"gpt-5.6-sol": {
			InputPricePerToken:             0.001,
			InputPricePerTokenPriority:     0.002,
			OutputPricePerToken:            0.002,
			OutputPricePerTokenPriority:    0.004,
			CacheCreationPricePerToken:     0.003,
			CacheReadPricePerToken:         0.0005,
			CacheReadPricePerTokenPriority: 0.001,
		},
	})

	for _, tt := range []struct {
		name string
		tier string
		want float64
	}{
		{name: "default", want: 0.265},
		{name: "priority", tier: "priority", want: 0.53},
		{name: "fast", tier: "fast", want: 0.53},
		{name: "flex", tier: "flex", want: 0.1325},
	} {
		t.Run(tt.name, func(t *testing.T) {
			usageLog := &UsageLog{}
			if tt.tier != "" {
				usageLog.ServiceTier = &tt.tier
			}

			applyAccountStatsCost(context.Background(), usageLog, cs, bs, 1, 10, "gpt-5.6-sol", "gpt-5.6-sol", tokens, 999)

			require.NotNil(t, usageLog.AccountStatsCost)
			require.InDelta(t, tt.want, *usageLog.AccountStatsCost, 1e-12)
		})
	}

	missing := &UsageLog{}
	applyAccountStatsCost(context.Background(), missing, cs, newTestBillingServiceWithPrices(map[string]*ModelPricing{}), 1, 10, "totally-unknown-model", "totally-unknown-model", tokens, 999)
	require.Nil(t, missing.AccountStatsCost)
}

func TestAccountStatsCost_ServiceTierDoesNotOverrideCustomOrChannelCost(t *testing.T) {
	priority := "priority"
	tokens := UsageTokens{InputTokens: 100, OutputTokens: 50}
	channel := &Channel{
		ID:                         1,
		Status:                     StatusActive,
		ApplyPricingToAccountStats: true,
		AccountStatsPricingRules: []AccountStatsPricingRule{{
			AccountIDs: []int64{1},
			Pricing: []ChannelModelPricing{{
				Platform:    "openai",
				Models:      []string{"gpt-5.6-sol"},
				InputPrice:  f64p(0.01),
				OutputPrice: f64p(0.02),
			}},
		}},
	}
	cs := newTestChannelServiceForStats(t, channel, 10, "openai")

	customLog := &UsageLog{ServiceTier: &priority}
	applyAccountStatsCost(context.Background(), customLog, cs, nil, 1, 10, "gpt-5.6-sol", "gpt-5.6-sol", tokens, 99)
	require.NotNil(t, customLog.AccountStatsCost)
	require.InDelta(t, 2.0, *customLog.AccountStatsCost, 1e-12)

	channel.AccountStatsPricingRules = nil
	channelLog := &UsageLog{ServiceTier: &priority}
	applyAccountStatsCost(context.Background(), channelLog, cs, nil, 1, 10, "gpt-5.6-sol", "gpt-5.6-sol", tokens, 0.75)
	require.NotNil(t, channelLog.AccountStatsCost)
	require.InDelta(t, 0.75, *channelLog.AccountStatsCost, 1e-12)
}

func task15ForwardOpenAIPassthrough(t *testing.T, stream bool, contentType string, upstreamBody string) *OpenAIForwardResult {
	t.Helper()
	gin.SetMode(gin.TestMode)
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{contentType}, "x-request-id": []string{"rid-task15"}},
		Body:       io.NopCloser(strings.NewReader(upstreamBody)),
	}}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := &Account{ID: 62, Name: "apikey-pass", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 1, Credentials: map[string]any{"api_key": "sk-task15", "base_url": "https://api.openai.com"}, Extra: map[string]any{"openai_passthrough": true}, Status: StatusActive, Schedulable: true}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5.6-sol","stream":` + task15BoolJSON(stream) + `,"input":[{"type":"message","role":"user","content":"hi"}]}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(string(body)))

	result, err := svc.Forward(context.Background(), c, account, body)
	require.NoError(t, err)
	require.NotNil(t, result)
	return result
}

func task15BoolJSON(v bool) string {
	if v {
		return "true"
	}
	return "false"
}
