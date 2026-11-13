package service

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestCurrentClaudePricingFallbacksAndOverrides(t *testing.T) {
	for _, tc := range []struct {
		model                            string
		input, output, read, write, hour float64
	}{
		{"claude-fable-5-2", 8, 40, .2, 10, 16},
		{"claude-mythos-5-2", 8, 40, .2, 10, 16},
		{"claude-opus-5-5", 4, 20, .2, 5, 8},
		{"claude-sonnet-5-5", 2, 10, .1, 2.5, 4},
		{"claude-haiku-5-5", .1, .5, .01, .125, .2},
	} {
		t.Run(tc.model, func(t *testing.T) {
			for _, prefix := range []string{"", "models/", "anthropic/", "global.anthropic."} {
				for _, catalog := range []*PricingService{nil, {pricingData: map[string]*LiteLLMModelPricing{}}} {
					p, err := NewBillingService(&config.Config{}, catalog).GetModelPricing(prefix + tc.model)
					require.NoError(t, err)
					require.InDelta(t, tc.input/1e6, p.InputPricePerToken, 1e-12)
					require.InDelta(t, tc.output/1e6, p.OutputPricePerToken, 1e-12)
					require.InDelta(t, tc.read/1e6, p.CacheReadPricePerToken, 1e-12)
					require.InDelta(t, tc.write/1e6, p.CacheCreation5mPrice, 1e-12)
					require.InDelta(t, tc.hour/1e6, p.CacheCreation1hPrice, 1e-12)
					require.True(t, p.SupportsCacheBreakdown)
				}
			}
			custom := &LiteLLMModelPricing{OutputCostPerToken: 7e-6}
			catalog := &PricingService{pricingData: map[string]*LiteLLMModelPricing{tc.model: custom}}
			require.Same(t, custom, catalog.GetModelPricing(tc.model))
		})
	}
	for _, model := range []string{"claude-fable-5-20", "claude-fable-5-2-preview", "claude-sonnet-5-5-preview", "gpt-5.6-sol"} {
		require.Nil(t, currentClaudePricing(model), model)
	}
	p := currentClaudeBillingPricing("claude-haiku-5-5")
	require.Equal(t, 100000, p.LongContextInputThreshold)
	require.Equal(t, 5.0, p.LongContextInputMultiplier)
	require.Equal(t, 5.0, p.LongContextOutputMultiplier)
	require.Zero(t, currentClaudeBillingPricing("claude-fable-5-2").LongContextInputThreshold)
}
