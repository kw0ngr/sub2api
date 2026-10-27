package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestGPT61TerraModelEffortCatalogAndPricing(t *testing.T) {
	require.Contains(t, openai.DefaultModelIDs(), "gpt-6.1-terra")
	for _, effort := range []string{"none", "low", "medium", "high", "xhigh", "max"} {
		id := "openai/gpt_6.1_terra_" + effort
		require.True(t, openai.IsGPT61TerraModelSpelling(id))
		require.Equal(t, "gpt-6.1-terra", normalizeKnownOpenAICodexModel(id))
		require.Equal(t, "gpt-6.1-terra", NormalizeOpenAICompatRequestedModel(id))
		require.Equal(t, effort, normalizeOpenAIReasoningEffortForModel(effort, id))
		req := &apicompat.AnthropicRequest{Model: id}
		applyOpenAICompatModelNormalization(req)
		require.Equal(t, "gpt-6.1-terra", req.Model)
		require.Equal(t, effort, req.OutputConfig.Effort)
		out, err := apicompat.AnthropicToResponses(req)
		require.NoError(t, err)
		require.Equal(t, effort, out.Reasoning.Effort)
	}
	for _, id := range []string{"gpt-6.1-terra-minimal", "gpt-6.1-terra-pro", "gpt-5.6-terra"} {
		require.False(t, openai.IsGPT61TerraModelSpelling(id))
	}
	require.Equal(t, "max", normalizeOpenAIReasoningEffortForModel("max", "gpt-5.6-sol"))
	require.NoError(t, validateOpenAIModelReasoningCompatRequest([]byte(`{"reasoning":{"effort":"none"}}`), "gpt-6.1-terra"))
	require.True(t, shouldAutoInjectPromptCacheKeyForCompat("gpt-6.1-terra"))
	descriptor := buildLocalCodexModelFromSpec(localCodexModelSpec{Slug: "gpt-6.1-terra"}, 0)
	require.Equal(t, "GPT-6.1 Terra", descriptor.DisplayName)
	require.Equal(t, "medium", descriptor.DefaultReasoningLevel)
	var levels []string
	for _, level := range descriptor.SupportedReasoningLevels {
		levels = append(levels, level.Effort)
	}
	require.Equal(t, []string{"none", "low", "medium", "high", "xhigh", "max"}, levels)
	price := (&PricingService{}).GetModelPricing("gpt-6.1-terra")
	require.InDelta(t, 1e-6, price.InputCostPerToken, 1e-12)
	require.InDelta(t, .05e-6, price.CacheReadInputTokenCost, 1e-12)
	require.InDelta(t, 1.25e-6, price.CacheCreationInputTokenCost, 1e-12)
	require.InDelta(t, 5e-6, price.OutputCostPerToken, 1e-12)
	require.InDelta(t, 2e-6, price.InputCostPerTokenPriority, 1e-12)
	billing := NewBillingService(&config.Config{}, nil)
	cost, err := billing.CalculateCost("gpt-6.1-terra", UsageTokens{InputTokens: 272001, OutputTokens: 100}, 1)
	require.NoError(t, err)
	require.InDelta(t, .544002, cost.InputCost, 1e-9)
	require.InDelta(t, .00075, cost.OutputCost, 1e-9)
}

func TestGPT61TerraChatOnlyAccountRejectsToolsBeforeUpstream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 1,
		Credentials: map[string]any{"api_key": "test-key", "model_mapping": map[string]any{"public": "gpt-6.1-terra"}},
		Extra:       map[string]any{"openai_responses_supported": false}}
	for _, protocol := range []string{"chat", "responses", "messages"} {
		t.Run(protocol, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/"+protocol, nil)
			upstream := &httpUpstreamRecorder{}
			svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
			var err error
			switch protocol {
			case "chat":
				_, err = svc.ForwardAsChatCompletions(context.Background(), c, account, []byte(`{"model":"public","messages":[{"role":"user","content":"hi"}],"tools":[{"type":"function","function":{"name":"lookup","parameters":{"type":"object"}}}]}`), "", "")
			case "responses":
				_, err = svc.Forward(context.Background(), c, account, []byte(`{"model":"public","input":"hi","tools":[{"type":"function","name":"lookup","parameters":{"type":"object"}}]}`))
			case "messages":
				_, err = svc.ForwardAsAnthropic(context.Background(), c, account, []byte(`{"model":"public","max_tokens":64,"messages":[{"role":"user","content":"hi"}],"tools":[{"name":"lookup","input_schema":{"type":"object"}}]}`), "", "")
			}
			require.ErrorContains(t, err, "gpt-6.1-terra requires Responses")
			require.Equal(t, 400, rec.Code)
			require.Nil(t, upstream.lastReq)
		})
	}
}

func TestGPT61TerraMappedMessagesPreserveNoneAndMax(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, effort := range []string{"none", "max"} {
		upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: 400, Header: http.Header{},
			Body: io.NopCloser(strings.NewReader(`{"error":{"message":"stop after request capture"}}`))}}
		svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
		account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 1,
			Credentials: map[string]any{"api_key": "test-key", "model_mapping": map[string]any{"public": "gpt-6.1-terra"}}}
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
		_, err := svc.ForwardAsAnthropic(context.Background(), c, account,
			[]byte(`{"model":"public","max_tokens":64,"output_config":{"effort":"`+effort+`"},"messages":[{"role":"user","content":"hi"}]}`), "", "")
		require.Error(t, err)
		require.Equal(t, effort, gjson.GetBytes(upstream.lastBody, "reasoning.effort").String())
	}
}
