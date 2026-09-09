package service

import (
	"context"
	"strings"
)

const responseModelBillingCostEpsilon = 1e-12

func responseModelBillingDeclaration(source, responseModel string, conflict, mediaBilled bool) string {
	if source != BillingModelSourceResponse || conflict || mediaBilled {
		return ""
	}
	return strings.TrimSpace(responseModel)
}

func normalizedResponseBillingModel(model string) string {
	model = strings.ToLower(strings.TrimSpace(model))
	if model == "" {
		return ""
	}
	if normalized := normalizeKnownOpenAICodexModel(model); normalized != "" {
		return normalized
	}
	return model
}

func responseModelBillingAdoptable(baseline, response *CostBreakdown, baselineChannelPriced, responseChannelPriced bool) bool {
	if baseline == nil || response == nil {
		return false
	}
	if response.TotalCost > baseline.TotalCost+responseModelBillingCostEpsilon {
		return false
	}
	if response.TotalCost <= 0 && baseline.TotalCost > 0 {
		return false
	}
	return !baselineChannelPriced || responseChannelPriced
}

func (s *GatewayService) hasIdentifiedResponseModelPricing(ctx context.Context, model string, apiKey *APIKey) (bool, bool) {
	if s == nil || s.billingService == nil || strings.TrimSpace(model) == "" {
		return false, false
	}
	if s.resolveChannelPricing(ctx, model, apiKey) != nil {
		return true, true
	}
	return s.billingService.HasIdentifiedTokenPricing(model), false
}

func (s *OpenAIGatewayService) hasIdentifiedOpenAIResponsePricing(ctx context.Context, model string, apiKey *APIKey) (bool, bool) {
	if s == nil || s.billingService == nil || strings.TrimSpace(model) == "" {
		return false, false
	}
	if s.resolveOpenAIChannelPricing(ctx, model, apiKey) != nil {
		return true, true
	}
	return s.billingService.HasIdentifiedTokenPricing(model), false
}

func (s *OpenAIGatewayService) resolveOpenAIChannelPricing(ctx context.Context, model string, apiKey *APIKey) *ResolvedPricing {
	if s == nil || s.resolver == nil || apiKey == nil || apiKey.Group == nil {
		return nil
	}
	gid := apiKey.Group.ID
	resolved := s.resolver.Resolve(ctx, PricingInput{Model: model, GroupID: &gid})
	if resolved != nil && resolved.Source == PricingSourceChannel {
		return resolved
	}
	return nil
}

func firstUsageBillingModel(models []string) string {
	if len(models) == 0 {
		return ""
	}
	return strings.TrimSpace(models[0])
}

func usageBillingModelCandidates(model string) []string {
	model = strings.TrimSpace(model)
	if model == "" {
		return nil
	}
	return []string{model}
}
