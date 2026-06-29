package service

import (
	"context"
	"strings"
)

type ModelAvailabilityDiagnosis struct {
	HasAccountsInPool bool
	HasModelSupport   bool
}

type ModelAvailabilityDiagnoser interface {
	DiagnoseModelAvailabilityForPlatform(ctx context.Context, groupID *int64, requestedModel string, platform string) ModelAvailabilityDiagnosis
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

	accounts, _, err := s.listSchedulableAccounts(ctx, groupID, platform, false)
	if err != nil {
		return availableByDefault()
	}

	diag := ModelAvailabilityDiagnosis{}
	for i := range accounts {
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
