package service

import (
	"context"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

type ModelAvailabilityDiagnosis struct {
	HasAccountsInPool bool
	HasModelSupport   bool
}

type ModelAvailabilityDiagnoser interface {
	DiagnoseModelAvailabilityForPlatform(ctx context.Context, groupID *int64, requestedModel string, platform string) ModelAvailabilityDiagnosis
}

type modelAvailabilityCandidateRepository interface {
	ListModelAvailabilityCandidates(ctx context.Context, groupID *int64, platforms []string, includeGrouped bool) ([]Account, error)
}

func listModelAvailabilityCandidates(ctx context.Context, repo AccountRepository, groupID *int64, platforms []string, includeGrouped bool) ([]Account, bool) {
	provider, ok := repo.(modelAvailabilityCandidateRepository)
	if !ok {
		return nil, false
	}
	accounts, err := provider.ListModelAvailabilityCandidates(ctx, groupID, platforms, includeGrouped)
	return accounts, err == nil
}

func (s *GatewayService) DiagnoseModelAvailabilityForPlatform(
	ctx context.Context,
	groupID *int64,
	requestedModel string,
	platform string,
) ModelAvailabilityDiagnosis {
	if s == nil {
		return availableByDefault()
	}
	requestedModel = strings.TrimSpace(requestedModel)
	if requestedModel == "" || strings.TrimSpace(platform) == "" {
		return availableByDefault()
	}

	if s.accountRepo == nil {
		return availableByDefault()
	}

	useMixed := platform == PlatformAnthropic || platform == PlatformGemini
	platforms := []string{platform}
	if useMixed {
		platforms = mixedSchedulingPlatformsForGateway(platform)
	}
	queryGroupID := groupID
	includeGrouped := false
	if useMixed {
		if groupID == nil && s.cfg != nil && s.cfg.RunMode == config.RunModeSimple {
			includeGrouped = true
		}
	} else if s.cfg != nil && s.cfg.RunMode == config.RunModeSimple {
		queryGroupID = nil
		includeGrouped = true
	}

	accounts, ok := listModelAvailabilityCandidates(ctx, s.accountRepo, queryGroupID, platforms, includeGrouped)
	if !ok {
		return availableByDefault()
	}

	diag := ModelAvailabilityDiagnosis{}
	for i := range accounts {
		if useMixed && !shouldIncludeMixedSchedulingAccount(platform, &accounts[i]) {
			continue
		}
		diag.HasAccountsInPool = true
		if s.isModelSupportedByAccountWithContext(ctx, &accounts[i], requestedModel) {
			diag.HasModelSupport = true
			return diag
		}
	}
	return diag
}

func availableByDefault() ModelAvailabilityDiagnosis {
	return ModelAvailabilityDiagnosis{HasAccountsInPool: true, HasModelSupport: true}
}
