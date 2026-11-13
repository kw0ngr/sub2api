package service

import "github.com/Wei-Shaw/sub2api/internal/pkg/claude"

// Fixed-model fallbacks from upstream 0.2.21; exact catalog/channel prices take precedence.
func currentClaudePricing(model string) *LiteLLMModelPricing {
	var input, output, read, write, hour float64
	switch {
	case claude.IsFable52(model), claude.IsMythos52(model):
		input, output, read, write, hour = 8, 40, .2, 10, 16
	case claude.IsOpus55(model):
		input, output, read, write, hour = 4, 20, .2, 5, 8
	case claude.IsSonnet55(model):
		input, output, read, write, hour = 2, 10, .1, 2.5, 4
	case claude.IsHaiku55(model):
		input, output, read, write, hour = .1, .5, .01, .125, .2
	default:
		return nil
	}
	p := &LiteLLMModelPricing{
		InputCostPerToken: input / 1e6, OutputCostPerToken: output / 1e6,
		CacheReadInputTokenCost: read / 1e6, CacheCreationInputTokenCost: write / 1e6,
		CacheCreationInputTokenCostAbove1hr: hour / 1e6,
		LiteLLMProvider:                     "anthropic", Mode: "chat", SupportsPromptCaching: true,
	}
	if claude.IsHaiku55(model) {
		p.LongContextInputTokenThreshold = 100000
		p.LongContextInputCostMultiplier = 5
		p.LongContextOutputCostMultiplier = 5
	}
	return p
}

func currentClaudeBillingPricing(model string) *ModelPricing {
	p := currentClaudePricing(model)
	if p == nil {
		return nil
	}
	return &ModelPricing{
		InputPricePerToken: p.InputCostPerToken, OutputPricePerToken: p.OutputCostPerToken,
		CacheReadPricePerToken: p.CacheReadInputTokenCost, CacheCreationPricePerToken: p.CacheCreationInputTokenCost,
		CacheCreation5mPrice: p.CacheCreationInputTokenCost, CacheCreation1hPrice: p.CacheCreationInputTokenCostAbove1hr,
		SupportsCacheBreakdown: true, LongContextInputThreshold: p.LongContextInputTokenThreshold,
		LongContextInputMultiplier: p.LongContextInputCostMultiplier, LongContextOutputMultiplier: p.LongContextOutputCostMultiplier,
	}
}
