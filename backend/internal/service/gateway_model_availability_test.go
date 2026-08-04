//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOpenAIDiagnoseModelAvailability_DeepSeekCooldownRemainsConfigured(t *testing.T) {
	groupID := int64(5)
	cooldownUntil := time.Now().Add(time.Hour)
	repo := &mockAccountRepoForPlatform{
		accounts: []Account{{
			ID:                     1908,
			Platform:               PlatformDeepSeek,
			Status:                 StatusActive,
			Schedulable:            true,
			TempUnschedulableUntil: &cooldownUntil,
			AccountGroups:          []AccountGroup{{GroupID: groupID}},
			Credentials: map[string]any{
				"model_mapping": map[string]any{"deepseek-v4-flash": "deepseek-v4-flash"},
			},
		}},
		accountsByID: map[int64]*Account{},
	}
	require.False(t, repo.accounts[0].IsSchedulable(), "precondition: normal scheduling must exclude the cooling account")
	svc := &OpenAIGatewayService{accountRepo: repo, cfg: testConfig()}

	diag := svc.DiagnoseModelAvailabilityForPlatform(
		context.Background(),
		&groupID,
		"deepseek-v4-flash",
		PlatformDeepSeek,
	)

	require.True(t, diag.HasAccountsInPool)
	require.True(t, diag.HasModelSupport, "temporary cooldown must produce 503 availability, not 404 model_not_found")
}

func TestGatewayDiagnoseModelAvailability_AnthropicCooldownRemainsConfigured(t *testing.T) {
	groupID := int64(6)
	cooldownUntil := time.Now().Add(time.Hour)
	repo := &mockAccountRepoForPlatform{accounts: []Account{{
		ID:               1917,
		Platform:         PlatformAnthropic,
		Status:           StatusActive,
		Schedulable:      true,
		RateLimitResetAt: &cooldownUntil,
		AccountGroups:    []AccountGroup{{GroupID: groupID}},
		Credentials: map[string]any{
			"model_mapping": map[string]any{"claude-sonnet-4-6": "claude-sonnet-4-6"},
		},
	}}}
	svc := &GatewayService{accountRepo: repo, cfg: testConfig()}

	diag := svc.DiagnoseModelAvailabilityForPlatform(
		context.Background(),
		&groupID,
		"claude-sonnet-4-6",
		PlatformAnthropic,
	)

	require.True(t, diag.HasAccountsInPool)
	require.True(t, diag.HasModelSupport)
}
