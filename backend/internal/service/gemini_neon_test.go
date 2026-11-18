package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGeminiNeonPricing(t *testing.T) {
	billing := NewBillingService(nil, nil)
	tokens := UsageTokens{InputTokens: 1_000_000, CacheReadTokens: 1_000_000, OutputTokens: 1_000_000}
	for _, tc := range []struct {
		tier                  string
		input, cached, output float64
	}{
		{"", 3, .15, 15},
		{"priority", 5.4, .27, 27},
		{"flex", 1.5, .075, 7.5},
	} {
		cost, err := billing.CalculateCostWithServiceTier("models/gemini-4-neon-xhigh", tokens, 1, tc.tier)
		require.NoError(t, err)
		require.InDelta(t, tc.input, cost.InputCost, 1e-9)
		require.InDelta(t, tc.cached, cost.CacheReadCost, 1e-9)
		require.InDelta(t, tc.output, cost.OutputCost, 1e-9)
	}
	require.InDelta(t, 3e-6, (&PricingService{}).GetModelPricing("gemini-4-neon").InputCostPerToken, 1e-12)
}

func TestGeminiNeonMessagesMaxUsesXHigh(t *testing.T) {
	cfg := convertClaudeGenerationConfig(map[string]any{
		"output_config": map[string]any{"effort": "max"},
	}, "gemini-4-neon")
	require.Equal(t, map[string]any{"thinkingLevel": "xhigh"}, cfg["thinkingConfig"])
}
