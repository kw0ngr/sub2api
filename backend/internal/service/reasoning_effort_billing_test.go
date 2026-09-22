package service

import (
	"context"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCalculateCostUnifiedAppliesReasoningEffortMultiplier(t *testing.T) {
	billing := NewBillingService(nil, nil)
	resolver := &ModelPricingResolver{}
	resolved := &ResolvedPricing{
		Mode: BillingModeToken,
		BasePricing: &ModelPricing{
			InputPricePerToken:  0.001,
			OutputPricePerToken: 0.002,
		},
		ReasoningEffortMultipliers: map[string]float64{"high": 2.5},
	}
	base := CostInput{
		Ctx: context.Background(), Model: "custom", Tokens: UsageTokens{InputTokens: 10, OutputTokens: 5},
		RateMultiplier: 1, Resolver: resolver, Resolved: resolved,
	}

	standard, err := billing.CalculateCostUnified(base)
	require.NoError(t, err)
	base.ReasoningEffort = "high"
	high, err := billing.CalculateCostUnified(base)
	require.NoError(t, err)

	require.InDelta(t, standard.TotalCost*2.5, high.TotalCost, 1e-12)
	require.InDelta(t, standard.ActualCost*2.5, high.ActualCost, 1e-12)
	require.InDelta(t, standard.InputCost*2.5, high.InputCost, 1e-12)
	require.InDelta(t, standard.OutputCost*2.5, high.OutputCost, 1e-12)
}

func TestCalculateCostUnifiedAppliesReasoningMultiplierToPerRequest(t *testing.T) {
	price := 0.4
	billing := NewBillingService(nil, nil)
	resolved := &ResolvedPricing{
		Mode: BillingModePerRequest, DefaultPerRequestPrice: price,
		ReasoningEffortMultipliers: map[string]float64{"max": 3},
	}
	cost, err := billing.CalculateCostUnified(CostInput{
		Ctx: context.Background(), Model: "custom", RequestCount: 2, RateMultiplier: 1.5,
		ReasoningEffort: "max", Resolver: &ModelPricingResolver{}, Resolved: resolved,
	})
	require.NoError(t, err)
	require.InDelta(t, price*2*3, cost.TotalCost, 1e-12)
	require.InDelta(t, price*2*1.5*3, cost.ActualCost, 1e-12)
}

func TestReasoningEffortMultiplierValidation(t *testing.T) {
	require.NoError(t, validateReasoningEffortMultipliers([]ChannelModelPricing{{
		Models: []string{"gpt-*"}, ReasoningEffortMultipliers: map[string]float64{"none": 0.5, "max": 3},
	}}))
	for name, multipliers := range map[string]map[string]float64{
		"unknown":   {"ultra": 2},
		"uppercase": {"HIGH": 2},
		"zero":      {"high": 0},
		"nan":       {"high": math.NaN()},
	} {
		t.Run(name, func(t *testing.T) {
			require.Error(t, validateReasoningEffortMultipliers([]ChannelModelPricing{{
				Models: []string{"gpt-*"}, ReasoningEffortMultipliers: multipliers,
			}}))
		})
	}
}

func TestChannelModelPricingCloneCopiesReasoningMultipliers(t *testing.T) {
	original := ChannelModelPricing{ReasoningEffortMultipliers: map[string]float64{"max": 3}}
	cloned := original.Clone()
	cloned.ReasoningEffortMultipliers["max"] = 2
	require.Equal(t, 3.0, original.ReasoningEffortMultipliers["max"])
}

func TestAccountStatsCustomPricingAppliesReasoningMultiplier(t *testing.T) {
	inputPrice := 0.001
	channel := &Channel{AccountStatsPricingRules: []AccountStatsPricingRule{{
		GroupIDs: []int64{7},
		Pricing: []ChannelModelPricing{{
			Models: []string{"custom"}, InputPrice: &inputPrice,
			ReasoningEffortMultipliers: map[string]float64{"high": 2},
		}},
	}}}
	standard := tryCustomRules(channel, 1, 7, "", "custom", UsageTokens{InputTokens: 10}, 1)
	high := tryCustomRules(channel, 1, 7, "", "custom", UsageTokens{InputTokens: 10}, 1, "high")
	require.NotNil(t, standard)
	require.NotNil(t, high)
	require.InDelta(t, *standard*2, *high, 1e-12)
}

func TestExtractGeminiReasoningEffortFromFinalBody(t *testing.T) {
	effort := extractGeminiReasoningEffortFromBody([]byte(`{"generationConfig":{"thinkingConfig":{"thinkingLevel":"HIGH"}}}`))
	require.NotNil(t, effort)
	require.Equal(t, "high", *effort)
	require.Nil(t, extractGeminiReasoningEffortFromBody([]byte(`{"generationConfig":{"thinkingConfig":{"thinkingBudget":8192}}}`)))
}
