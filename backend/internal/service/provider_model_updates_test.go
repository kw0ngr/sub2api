package service

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestCurrentProviderPricingOverridesStaleRemoteAndKeepsFlashAliases(t *testing.T) {
	svc := &PricingService{pricingData: map[string]*LiteLLMModelPricing{
		"deepseek-flash":   {InputCostPerToken: 99},
		"deepseek-v4-pro":  {InputCostPerToken: 99},
		"gemini-3.8-flash": {InputCostPerToken: 99},
	}}
	for _, model := range []string{"deepseek-flash", "deepseek-v4-flash", "deepseek-v4-flash-vision-exp", "deepseek/deepseek-flash"} {
		price := svc.GetModelPricing(model)
		require.NotNil(t, price, model)
		require.InDelta(t, .3e-6, price.InputCostPerToken, 1e-12)
		require.InDelta(t, .006e-6, price.CacheReadInputTokenCost, 1e-12)
		require.InDelta(t, 1.2e-6, price.OutputCostPerToken, 1e-12)
		require.NotNil(t, svc.GetIdentifiedModelPricing(model))
	}
	pro := svc.GetModelPricing("deepseek-v4-pro")
	require.InDelta(t, 1.32e-6, pro.InputCostPerToken, 1e-12)
	require.InDelta(t, 3.96e-6, pro.OutputCostPerToken, 1e-12)
	require.InDelta(t, .044e-6, pro.CacheReadInputTokenCost, 1e-12)
	for _, model := range []string{"gemini-3.6-flash", "gemini-3.7-flash", "gemini-3.8-flash", "gemini-3.8-flash-high"} {
		price := svc.GetModelPricing(model)
		require.InDelta(t, .75e-6, price.InputCostPerToken, 1e-12)
		require.InDelta(t, 3.75e-6, price.OutputCostPerToken, 1e-12)
		require.InDelta(t, 1.35e-6, price.InputCostPerTokenPriority, 1e-12)
	}
	require.Nil(t, currentProviderPricing("deepseek-unknown"))
}

func TestNanoBanana21ImageRecognitionDefaultSizeAndExactPrices(t *testing.T) {
	geminiSvc := &GeminiMessagesCompatService{}
	antigravitySvc := &AntigravityGatewayService{}
	billing := NewBillingService(&config.Config{}, nil)
	for _, model := range []string{"gemini-nano-banana-2.1", "models/gemini-nano-banana-2.1"} {
		require.True(t, isImageGenerationModel(model))
		require.Equal(t, "1K", geminiSvc.extractImageSize([]byte(`{}`), model))
		require.Equal(t, "1K", antigravitySvc.extractImageSize([]byte(`{}`), model))
		require.InDelta(t, .0336, billing.getDefaultImagePrice(model, "1K"), 1e-12)
		require.InDelta(t, .0504, billing.getDefaultImagePrice(model, "2K"), 1e-12)
		require.InDelta(t, .1134, billing.getDefaultImagePrice(model, "4K"), 1e-12)
		body := []byte(`{"generationConfig":{"imageConfig":{"imageSize":"4K"}}}`)
		require.Equal(t, "4K", geminiSvc.extractImageSize(body, model))
		require.Equal(t, "4K", antigravitySvc.extractImageSize(body, model))
	}
	require.False(t, isImageGenerationModel("gemini-nano-banana-2.10"))
	payload := createGeminiTestPayload("gemini-nano-banana-2.1", "")
	require.Equal(t, `["IMAGE"]`, gjson.GetBytes(payload, "generationConfig.responseModalities").Raw)
	require.Equal(t, "1K", gjson.GetBytes(payload, "generationConfig.imageConfig.imageSize").String())
	override := .2
	require.InDelta(t, override, billing.getImageUnitPrice("gemini-nano-banana-2.1", "1K", &ImagePriceConfig{Price1K: &override}), 1e-12)
}

func TestLatestGeminiCompatPreservesEffortAndDoesNotChangeOpenAISolMax(t *testing.T) {
	for _, effort := range []string{"low", "medium", "high", "max", "xhigh"} {
		body, err := json.Marshal(map[string]any{
			"model": "my-gemini", "max_tokens": 128, "temperature": .5, "top_p": .8,
			"output_config": map[string]any{"effort": effort},
			"messages":      []any{map[string]any{"role": "user", "content": "hi"}},
		})
		require.NoError(t, err)
		out, err := convertClaudeMessagesToGeminiGenerateContent(body, "gemini-3.8-flash")
		require.NoError(t, err)
		want := effort
		if effort == "max" || effort == "xhigh" {
			want = "high"
		}
		require.Equal(t, want, gjson.GetBytes(out, "generationConfig.thinkingConfig.thinkingLevel").String())
		require.False(t, gjson.GetBytes(out, "generationConfig.temperature").Exists())
		require.False(t, gjson.GetBytes(out, "generationConfig.topP").Exists())
	}
	body := []byte(`{"reasoning_effort":"max"}`)
	normalized, changed := normalizeGeminiOpenAIReasoningEffort(body, "gemini-3.8-flash")
	require.True(t, changed)
	require.Equal(t, "high", gjson.GetBytes(normalized, "reasoning_effort").String())
	for _, model := range []string{"gpt-6.1-sol", "gpt-5.6-sol", "unknown"} {
		normalized, changed := normalizeGeminiOpenAIReasoningEffort(body, model)
		require.False(t, changed)
		require.Equal(t, body, normalized)
	}
	none := []byte(`{"reasoning_effort":"none"}`)
	normalized, changed = normalizeGeminiOpenAIReasoningEffort(none, "gemini-3.8-flash")
	require.False(t, changed, "Google's OpenAI endpoint accepts none; do not change it based on native minimal restrictions")
	require.Equal(t, none, normalized)
}
