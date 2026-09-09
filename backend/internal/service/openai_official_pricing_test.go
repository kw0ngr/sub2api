package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

type task4OfficialPrice struct {
	model   string
	aliases []string
	input   float64
	cached  float64
	write   float64
	output  float64
}

func TestOpenAIOfficialPricingMatrix(t *testing.T) {
	cases := []task4OfficialPrice{
		{model: "gpt-6-astra", aliases: []string{"gpt-6-astra", "gpt-6", "openai/gpt-6", "provider/gpt-6-astra-max"}, input: 10e-6, cached: 1e-6, write: 12.5e-6, output: 50e-6},
		{model: "gpt-5.6-sol", aliases: []string{"gpt-5.6-sol", "gpt-5.6", "gpt5.6", "openai/gpt-5.6", "provider/gpt-5.6-max", "gpt-5.6-2026-07-09"}, input: 4e-6, cached: 0.4e-6, write: 5e-6, output: 20e-6},
		{model: "gpt-5.6-terra", aliases: []string{"gpt-5.6-terra", "openai/gpt-5.6-terra", "provider/gpt-5.6-terra-max", "gpt-5.6-terra-2026-07-09"}, input: 2e-6, cached: 0.2e-6, write: 2.5e-6, output: 12e-6},
		{model: "gpt-5.6-luna", aliases: []string{"gpt-5.6-luna", "openai/gpt-5.6-luna", "provider/gpt-5.6-luna-max", "gpt-5.6-luna-2026-07-09"}, input: 0.2e-6, cached: 0.02e-6, write: 0.25e-6, output: 1.2e-6},
	}
	dynamicPricing := task4DynamicPricingService(t)
	staticPricing := &PricingService{pricingData: map[string]*LiteLLMModelPricing{}}
	dynamicBilling := NewBillingService(nil, dynamicPricing)
	fallbackBilling := NewBillingService(nil, nil)

	for _, tc := range cases {
		t.Run(tc.model, func(t *testing.T) {
			for _, alias := range tc.aliases {
				t.Run(alias, func(t *testing.T) {
					assertTask4LiteLLMPrice(t, dynamicPricing.GetModelPricing(alias), tc)
					assertTask4LiteLLMPrice(t, staticPricing.GetModelPricing(alias), tc)

					dynamicPrice, err := dynamicBilling.GetModelPricing(alias)
					require.NoError(t, err)
					assertTask4BillingPrice(t, dynamicPrice, tc)
					fallbackPrice, err := fallbackBilling.GetModelPricing(alias)
					require.NoError(t, err)
					assertTask4BillingPrice(t, fallbackPrice, tc)
				})
			}

			for _, tier := range []struct {
				name       string
				multiplier float64
			}{
				{name: "", multiplier: 1},
				{name: "priority", multiplier: 2},
				{name: "fast", multiplier: 2},
				{name: "flex", multiplier: 0.5},
				{name: "batch", multiplier: 0.5},
			} {
				t.Run("tier/"+tier.name, func(t *testing.T) {
					cost, err := fallbackBilling.CalculateCostWithServiceTier(tc.model, task4TokenFixture(), 1, tier.name)
					require.NoError(t, err)
					assertTask4Cost(t, cost, task4TokenFixture(), tc, tier.multiplier, 1, 1)
				})
			}

			t.Run("long context boundary", func(t *testing.T) {
				boundary := UsageTokens{InputTokens: 271996, CacheCreationTokens: 3, CacheReadTokens: 4, OutputTokens: 5}
				boundaryCost, err := fallbackBilling.CalculateCost(tc.model, boundary, 1)
				require.NoError(t, err)
				assertTask4Cost(t, boundaryCost, boundary, tc, 1, 1, 1)

				above := UsageTokens{InputTokens: 271997, CacheCreationTokens: 3, CacheReadTokens: 4, OutputTokens: 5}
				aboveCost, err := dynamicBilling.CalculateCost(tc.model, above, 1)
				require.NoError(t, err)
				assertTask4Cost(t, aboveCost, above, tc, 1, 2, 1.5)
			})
		})
	}

	for _, model := range []string{"gpt-6-other", "gpt-6-astral", "gpt-5.6-preview"} {
		t.Run("unknown/"+model, func(t *testing.T) {
			require.Nil(t, dynamicPricing.GetModelPricing(model))
			require.Nil(t, staticPricing.GetModelPricing(model))
			pricing, err := fallbackBilling.GetModelPricing(model)
			require.ErrorIs(t, err, ErrModelPricingUnavailable)
			require.Nil(t, pricing)
		})
	}
}

func task4DynamicPricingService(t *testing.T) *PricingService {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "..", "resources", "model-pricing", "model_prices_and_context_window.json"))
	require.NoError(t, err)
	svc := &PricingService{}
	data, err := svc.parsePricingData(body)
	require.NoError(t, err)
	svc.pricingData = data
	return svc
}

func task4TokenFixture() UsageTokens {
	return UsageTokens{InputTokens: 1000, CacheCreationTokens: 2000, CacheReadTokens: 3000, OutputTokens: 4000}
}

func assertTask4LiteLLMPrice(t *testing.T, got *LiteLLMModelPricing, want task4OfficialPrice) {
	t.Helper()
	require.NotNil(t, got)
	require.InDelta(t, want.input, got.InputCostPerToken, 1e-12)
	require.InDelta(t, want.cached, got.CacheReadInputTokenCost, 1e-12)
	require.InDelta(t, want.write, got.CacheCreationInputTokenCost, 1e-12)
	require.InDelta(t, want.output, got.OutputCostPerToken, 1e-12)
}

func assertTask4BillingPrice(t *testing.T, got *ModelPricing, want task4OfficialPrice) {
	t.Helper()
	require.NotNil(t, got)
	require.InDelta(t, want.input, got.InputPricePerToken, 1e-12)
	require.InDelta(t, want.cached, got.CacheReadPricePerToken, 1e-12)
	require.InDelta(t, want.write, got.CacheCreationPricePerToken, 1e-12)
	require.InDelta(t, want.output, got.OutputPricePerToken, 1e-12)
	require.Equal(t, 272000, got.LongContextInputThreshold)
	require.InDelta(t, 2.0, got.LongContextInputMultiplier, 1e-12)
	require.InDelta(t, 1.5, got.LongContextOutputMultiplier, 1e-12)
}

func assertTask4Cost(t *testing.T, got *CostBreakdown, tokens UsageTokens, want task4OfficialPrice, tierMultiplier float64, inputMultiplier float64, outputMultiplier float64) {
	t.Helper()
	expectedInput := float64(tokens.InputTokens) * want.input * tierMultiplier * inputMultiplier
	expectedWrite := float64(tokens.CacheCreationTokens) * want.write * tierMultiplier * inputMultiplier
	expectedCached := float64(tokens.CacheReadTokens) * want.cached * tierMultiplier * inputMultiplier
	expectedOutput := float64(tokens.OutputTokens) * want.output * tierMultiplier * outputMultiplier
	require.InDelta(t, expectedInput, got.InputCost, 1e-10)
	require.InDelta(t, expectedWrite, got.CacheCreationCost, 1e-10)
	require.InDelta(t, expectedCached, got.CacheReadCost, 1e-10)
	require.InDelta(t, expectedOutput, got.OutputCost, 1e-10)
	require.InDelta(t, expectedInput+expectedWrite+expectedCached+expectedOutput, got.TotalCost, 1e-10)
}
