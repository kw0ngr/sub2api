package service

import (
	"context"
	"encoding/json"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGPT62RoutingMetadataReasoningAndPricing(t *testing.T) {
	for _, model := range []string{"gpt-6.2-sol", "gpt-6.2-astra"} {
		t.Run(model, func(t *testing.T) {
			for _, alias := range []string{model, "openai/" + model + "-max", strings.ToUpper(strings.ReplaceAll(model, "-", "_"))} {
				require.Equal(t, model, normalizeKnownOpenAICodexModel(alias))
				require.Equal(t, model, NormalizeOpenAICompatRequestedModel(alias))
				require.Equal(t, "max", normalizeOpenAIReasoningEffortForModel("max", alias))
				require.True(t, openai.RequiresResponsesForToolCalls(alias))
			}
			require.Empty(t, normalizeKnownOpenAICodexModel(model+"-preview"))
			for _, effort := range []string{"low", "medium", "high", "xhigh", "max"} {
				require.NoError(t, openai.ValidateModelReasoningEffort(model, effort))
			}
			for _, effort := range []string{"none", "minimal"} {
				require.Error(t, openai.ValidateModelReasoningEffort(model, effort))
				require.Error(t, validateOpenAIModelReasoningCompatRequest([]byte(`{"thinking":{"type":"disabled"}}`), model))
			}
			meta := buildLocalCodexModelFromSpec(localCodexModelSpec{Slug: model, MetadataID: model, ForceAPIKey: true}, 0)
			require.Equal(t, model, meta.Slug)
			require.Equal(t, 1050000, meta.ContextWindow)
			levels := []string{}
			for _, l := range meta.SupportedReasoningLevels {
				levels = append(levels, l.Effort)
			}
			require.Equal(t, []string{"low", "medium", "high", "xhigh", "max"}, levels)
			require.True(t, meta.SupportsSearchTool)
			require.True(t, meta.SupportsImageDetailOriginal)
			require.Equal(t, []string{"low", "max"}, normalizeSnapshotReasoningLevelsForModel(model, []string{"none", "max", "low"}))
			cc, err := apicompat.ChatCompletionsToResponses(&apicompat.ChatCompletionsRequest{Model: model, ReasoningEffort: "max", Messages: []apicompat.ChatMessage{{Role: "user", Content: json.RawMessage(`"hi"`)}}})
			require.NoError(t, err)
			require.Equal(t, "max", cc.Reasoning.Effort)
			anth, err := apicompat.AnthropicToResponses(&apicompat.AnthropicRequest{Model: model, MaxTokens: 1024, Messages: []apicompat.AnthropicMessage{{Role: "user", Content: json.RawMessage(`"hi"`)}}, OutputConfig: &apicompat.AnthropicOutputConfig{Effort: "max"}})
			require.NoError(t, err)
			require.Equal(t, "max", anth.Reasoning.Effort)
			p := (&PricingService{}).GetModelPricing(model)
			require.NotNil(t, p)
			input, cached, write, output := 2e-6, .1e-6, 2.5e-6, 10e-6
			if model == "gpt-6.2-astra" {
				input, cached, write, output = 8e-6, .4e-6, 10e-6, 40e-6
			}
			require.InDelta(t, input, p.InputCostPerToken, 1e-12)
			require.InDelta(t, cached, p.CacheReadInputTokenCost, 1e-12)
			require.InDelta(t, write, p.CacheCreationInputTokenCost, 1e-12)
			require.InDelta(t, output, p.OutputCostPerToken, 1e-12)
			billing := NewBillingService(nil, nil)
			b, err := billing.GetModelPricing(model)
			require.NoError(t, err)
			require.Equal(t, input, b.InputPricePerToken)
			require.Equal(t, 272000, b.LongContextInputThreshold)
			cost, err := billing.CalculateCost(model, UsageTokens{InputTokens: 300000, OutputTokens: 1000}, 1)
			require.NoError(t, err)
			require.InDelta(t, 300000*input*2, cost.InputCost, 1e-9)
			require.InDelta(t, 1000*output*1.5, cost.OutputCost, 1e-9)
		})
	}
}

func TestGPT62ChatToolCallsUseResponsesAndPreserveMax(t *testing.T) {
	for _, model := range []string{"gpt-6.2-sol", "gpt-6.2-astra"} {
		cfg := &config.Config{}
		up := &httpUpstreamRecorder{resp: &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader("data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_test\",\"status\":\"completed\",\"model\":\"" + model + "\",\"output\":[{\"type\":\"message\",\"role\":\"assistant\",\"content\":[{\"type\":\"output_text\",\"text\":\"OK\"}]}],\"usage\":{\"input_tokens\":1,\"output_tokens\":1}}}\n\n"))}}
		svc := &OpenAIGatewayService{cfg: cfg, httpUpstream: up}
		account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 1, Credentials: map[string]any{"api_key": "sk-test", "base_url": "https://example.com/v1"}, Extra: map[string]any{"openai_responses_mode": "force_responses"}}
		b := []byte(`{"model":"` + model + `","messages":[{"role":"user","content":"hi"}],"reasoning_effort":"max","temperature":0.7,"tools":[{"type":"function","function":{"name":"lookup","parameters":{"type":"object","properties":{}}}}]}`)
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(b)))
		_, err := svc.ForwardAsChatCompletions(context.Background(), c, account, b, "", "")
		require.NoError(t, err)
		require.True(t, strings.HasSuffix(up.lastReq.URL.Path, "/responses"))
		require.Equal(t, model, gjson.GetBytes(up.lastBody, "model").String())
		require.Equal(t, "max", gjson.GetBytes(up.lastBody, "reasoning.effort").String())
		require.False(t, gjson.GetBytes(up.lastBody, "temperature").Exists())
	}
}
