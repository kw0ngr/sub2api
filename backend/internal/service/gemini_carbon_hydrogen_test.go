package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestGeminiCarbonHydrogenEffortDoesNotDowngradeHydrogen(t *testing.T) {
	for _, tc := range []struct{ model, effort, want string }{
		{"gemini-4-carbon", "max", "high"},
		{"gemini-4-carbon", "xhigh", "high"},
		{"gemini-4-hydrogen", "max", "xhigh"},
		{"gemini-4-hydrogen", "xhigh", "xhigh"},
		{"gemini-4-hydrogen", " XHIGH ", "xhigh"},
		{"models/gemini-4-hydrogen", "x-high", "xhigh"},
		{"gemini-4-hydrogen", "low", "low"},
		{"gemini-4-carbon", "none", "none"},
		{"gpt-5.6-sol", "max", "max"},
	} {
		body := []byte(`{"reasoning_effort":"` + tc.effort + `","messages":[{"role":"user","content":"hi"}]}`)
		out, _ := normalizeGeminiOpenAIReasoningEffort(body, tc.model)
		require.Equal(t, tc.want, gjson.GetBytes(out, "reasoning_effort").String(), tc)
	}
	for _, model := range []string{"gemini-4-carbon", "gemini-4-hydrogen"} {
		req := map[string]any{"temperature": .5, "top_p": .8, "output_config": map[string]any{"effort": "max"}}
		want := "high"
		if model == "gemini-4-hydrogen" {
			want = "xhigh"
		}
		out := convertClaudeGenerationConfig(req, model)
		require.Equal(t, want, out["thinkingConfig"].(map[string]any)["thinkingLevel"])
		require.Equal(t, .5, out["temperature"])
	}
	effort := extractGeminiReasoningEffortFromBody([]byte(`{"generationConfig":{"thinkingConfig":{"thinkingLevel":"XHIGH"}}}`))
	require.NotNil(t, effort)
	require.Equal(t, "xhigh", *effort)
}

func TestGeminiCarbonHydrogenFallbackPricing(t *testing.T) {
	deadline := time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC)
	require.InDelta(t, 2e-6, geminiCarbonPricing(deadline.Add(-time.Second)).InputCostPerToken, 1e-12)
	require.InDelta(t, 4e-6, geminiCarbonPricing(deadline).InputCostPerToken, 1e-12)
	require.InDelta(t, 10e-6, geminiCarbonPricing(deadline.Add(-time.Second)).InputCostPerTokenPriority, 1e-12)
	for _, model := range []string{"gemini-4-carbon", "gemini-4-hydrogen", "models/gemini-4-hydrogen-xhigh"} {
		p := (&PricingService{}).GetModelPricing(model)
		require.NotNil(t, p, model)
		fallback := NewBillingService(nil, nil).getFallbackPricing(model)
		require.NotNil(t, fallback, model)
		require.Equal(t, p.InputCostPerToken, fallback.InputPricePerToken)
		require.Equal(t, p.OutputCostPerToken, fallback.OutputPricePerToken)
		require.Equal(t, p.CacheReadInputTokenCost, fallback.CacheReadPricePerToken)
		require.Equal(t, p.OutputCostPerTokenPriority, fallback.OutputPricePerTokenPriority)
	}
	require.InDelta(t, 1.5e-6, (&PricingService{}).GetModelPricing("gemini-4-hydrogen").InputCostPerToken, 1e-12)
}

func TestGeminiOpenAIRequestRemovesZCodeThinkingAliases(t *testing.T) {
	body := []byte(`{"model":"gemini-4-hydrogen","reasoning_effort":"none","reasoning":{"effort":"none"},"thinking":{"type":"disabled"},"enable_thinking":false}`)
	out, changed := normalizeGeminiOpenAIReasoningEffort(body, "gemini-4-hydrogen")
	require.True(t, changed)
	require.Equal(t, "none", gjson.GetBytes(out, "reasoning_effort").String())
	for _, field := range []string{"reasoning", "thinking", "enable_thinking"} {
		require.False(t, gjson.GetBytes(out, field).Exists(), field)
	}

	out, changed = normalizeGeminiOpenAIReasoningEffort([]byte(`{"reasoning":{"effort":"max"}}`), "gemini-4-hydrogen")
	require.True(t, changed)
	require.Equal(t, "xhigh", gjson.GetBytes(out, "reasoning_effort").String())
	require.False(t, gjson.GetBytes(out, "reasoning").Exists())

	out, changed = normalizeGeminiOpenAIReasoningEffort([]byte(`{"thinking":{"type":"disabled"},"enable_thinking":true}`), "gemini-4-carbon")
	require.True(t, changed)
	require.Equal(t, "none", gjson.GetBytes(out, "reasoning_effort").String(), "explicit thinking.type wins over the legacy boolean")
}
