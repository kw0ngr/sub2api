package service

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFallbackPricingContainsLatestGeminiModels(t *testing.T) {
	t.Parallel()

	_, sourceFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	pricingPath := filepath.Join(filepath.Dir(sourceFile), "..", "..", "resources", "model-pricing", "model_prices_and_context_window.json")
	body, err := os.ReadFile(pricingPath)
	require.NoError(t, err)

	pricingData, err := (&PricingService{}).parsePricingData(body)
	require.NoError(t, err)

	tests := []struct {
		model         string
		input         float64
		output        float64
		cacheRead     float64
		priorityInput float64
		priorityOut   float64
	}{
		{model: "gemini-3.6-flash", input: 1.50e-6, output: 7.50e-6, cacheRead: 0.15e-6, priorityInput: 2.70e-6, priorityOut: 13.50e-6},
		{model: "gemini-3.5-flash-lite", input: 0.30e-6, output: 2.50e-6, cacheRead: 0.03e-6, priorityInput: 0.54e-6, priorityOut: 4.50e-6},
		{model: "gemini-3.5-flash", input: 1.50e-6, output: 9.00e-6, cacheRead: 0.15e-6, priorityInput: 2.70e-6, priorityOut: 16.20e-6},
		{model: "gemini-3.1-flash-lite", input: 0.25e-6, output: 1.50e-6, cacheRead: 0.025e-6, priorityInput: 0.45e-6, priorityOut: 2.70e-6},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			pricing := pricingData[tt.model]
			require.NotNil(t, pricing)
			require.InDelta(t, tt.input, pricing.InputCostPerToken, 1e-12)
			require.InDelta(t, tt.output, pricing.OutputCostPerToken, 1e-12)
			require.InDelta(t, tt.cacheRead, pricing.CacheReadInputTokenCost, 1e-12)
			require.InDelta(t, tt.priorityInput, pricing.InputCostPerTokenPriority, 1e-12)
			require.InDelta(t, tt.priorityOut, pricing.OutputCostPerTokenPriority, 1e-12)
			require.True(t, pricing.SupportsServiceTier)
		})
	}
}
