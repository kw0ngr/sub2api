package service

import (
	"context"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

func (s *OpenAIGatewayService) DiagnoseModelAvailabilityForPlatform(
	ctx context.Context,
	groupID *int64,
	requestedModel string,
	platform string,
) ModelAvailabilityDiagnosis {
	if s == nil {
		return availableByDefault()
	}
	requestedModel = strings.TrimSpace(requestedModel)
	if requestedModel == "" {
		return availableByDefault()
	}
	if s.accountRepo == nil {
		return availableByDefault()
	}

	platform = normalizeOpenAICompatiblePlatform(platform)
	queryGroupID := groupID
	includeGrouped := false
	if s.cfg != nil && s.cfg.RunMode == config.RunModeSimple {
		queryGroupID = nil
		includeGrouped = true
	}
	accounts, ok := listModelAvailabilityCandidates(ctx, s.accountRepo, queryGroupID, []string{platform}, includeGrouped)
	if !ok {
		return availableByDefault()
	}

	diag := ModelAvailabilityDiagnosis{}
	allResourceUnavailable := true
	for i := range accounts {
		if platform == PlatformGLM && !accounts[i].IsGLMOpenAICompatible() {
			continue
		}
		diag.HasAccountsInPool = true
		if accounts[i].IsModelSupported(requestedModel) {
			diag.HasModelSupport = true
			allResourceUnavailable = allResourceUnavailable && accountHasGLMResourceUnavailable(&accounts[i])
		}
	}
	diag.ResourceUnavailable = diag.HasModelSupport && allResourceUnavailable
	return diag
}
