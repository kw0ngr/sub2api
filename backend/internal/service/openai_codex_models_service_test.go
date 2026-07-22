package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

type localCodexModelsAccountRepoStub struct {
	AccountRepository
	accounts []Account
}

func (s localCodexModelsAccountRepoStub) ListSchedulableByGroupIDAndPlatform(_ context.Context, _ int64, platform string) ([]Account, error) {
	return s.filterPlatform(platform), nil
}

func (s localCodexModelsAccountRepoStub) ListSchedulableByPlatform(_ context.Context, platform string) ([]Account, error) {
	return s.filterPlatform(platform), nil
}

func (s localCodexModelsAccountRepoStub) filterPlatform(platform string) []Account {
	result := make([]Account, 0, len(s.accounts))
	for _, account := range s.accounts {
		if account.Platform == platform {
			result = append(result, account)
		}
	}
	return result
}

func TestSelectCodexModelsAccountHydratesSchedulerSnapshotCredentials(t *testing.T) {
	groupID := int64(11)
	const accountID int64 = 2103

	snapshotAccount := &Account{
		ID:          accountID,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
	}
	hydratedAccount := &Account{
		ID:          accountID,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Credentials: map[string]any{
			"access_token": "oauth-access-token",
		},
	}
	snapshotCache := &openAISnapshotCacheStub{
		snapshotAccounts: []*Account{snapshotAccount},
		accountsByID:     map[int64]*Account{accountID: hydratedAccount},
	}
	service := &OpenAIGatewayService{
		schedulerSnapshot: &SchedulerSnapshotService{cache: snapshotCache},
	}

	account, err := service.SelectCodexModelsAccount(context.Background(), &groupID)

	require.NoError(t, err)
	require.NotNil(t, account)
	require.Equal(t, accountID, account.ID)
	require.Equal(t, "oauth-access-token", account.GetOpenAIAccessToken())
}

func TestSelectCodexModelsAccountSkipsHydratedUnschedulableAccount(t *testing.T) {
	groupID := int64(11)

	snapshotCache := &openAISnapshotCacheStub{
		snapshotAccounts: []*Account{
			{ID: 2103, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true},
			{ID: 2104, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true},
		},
		accountsByID: map[int64]*Account{
			2103: {
				ID:          2103,
				Platform:    PlatformOpenAI,
				Type:        AccountTypeOAuth,
				Status:      StatusActive,
				Schedulable: false,
				Credentials: map[string]any{"access_token": "stale-token"},
			},
			2104: {
				ID:          2104,
				Platform:    PlatformOpenAI,
				Type:        AccountTypeOAuth,
				Status:      StatusActive,
				Schedulable: true,
				Credentials: map[string]any{"access_token": "fresh-token"},
			},
		},
	}
	service := &OpenAIGatewayService{
		schedulerSnapshot: &SchedulerSnapshotService{cache: snapshotCache},
	}

	account, err := service.SelectCodexModelsAccount(context.Background(), &groupID)

	require.NoError(t, err)
	require.NotNil(t, account)
	require.Equal(t, int64(2104), account.ID)
}

func TestBuildLocalCodexModelsManifestUsesAPIKeyMappingsAndAdvertisesSolMax(t *testing.T) {
	groupID := int64(11)
	svc := &OpenAIGatewayService{accountRepo: localCodexModelsAccountRepoStub{accounts: []Account{
		{
			ID:          581,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Credentials: map[string]any{
				"model_mapping": map[string]any{
					"gpt-4.1":     "gpt-4.1",
					"gpt-5.4":     "gpt-5.4",
					"gpt-5.6-sol": "gpt-5.6-sol",
				},
			},
		},
	}}}

	manifest, err := svc.BuildLocalCodexModelsManifest(context.Background(), &groupID, "")
	require.NoError(t, err)
	require.NotNil(t, manifest)
	require.NotEmpty(t, manifest.ETag)
	require.False(t, manifest.NotModified)

	var envelope struct {
		Models []localCodexModel `json:"models"`
	}
	require.NoError(t, json.Unmarshal(manifest.Body, &envelope))
	require.Len(t, envelope.Models, 3)
	require.Equal(t, "gpt-5.6-sol", envelope.Models[0].Slug)

	sol := envelope.Models[0]
	require.Equal(t, "GPT-5.6 Sol", sol.DisplayName)
	require.Equal(t, "low", sol.DefaultReasoningLevel)
	require.Equal(t, 272000, sol.ContextWindow)
	require.Equal(t, 1000000, sol.MaxContextWindow)
	efforts := make([]string, 0, len(sol.SupportedReasoningLevels))
	for _, level := range sol.SupportedReasoningLevels {
		efforts = append(efforts, level.Effort)
	}
	require.Equal(t, []string{"low", "medium", "high", "xhigh", "max"}, efforts)

	notModified, err := svc.BuildLocalCodexModelsManifest(context.Background(), &groupID, manifest.ETag)
	require.NoError(t, err)
	require.True(t, notModified.NotModified)
	require.Equal(t, manifest.ETag, notModified.ETag)
	require.Empty(t, notModified.Body)
}

func TestBuildLocalCodexModelsManifestFallsBackToDefaultCatalogForWildcardAPIKey(t *testing.T) {
	groupID := int64(11)
	svc := &OpenAIGatewayService{accountRepo: localCodexModelsAccountRepoStub{accounts: []Account{
		{
			ID:          582,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Credentials: map[string]any{"model_mapping": map[string]any{"gpt-*": "gpt-*"}},
		},
	}}}

	manifest, err := svc.BuildLocalCodexModelsManifest(context.Background(), &groupID, "")
	require.NoError(t, err)
	require.Contains(t, string(manifest.Body), `"slug":"gpt-5.6-sol"`)
	require.NotContains(t, string(manifest.Body), `"slug":"gpt-*"`)
}

func TestBuildLocalCodexModelsManifestRequiresAPIKeyAccount(t *testing.T) {
	groupID := int64(11)
	svc := &OpenAIGatewayService{accountRepo: localCodexModelsAccountRepoStub{accounts: []Account{
		{
			ID:          2103,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeOAuth,
			Status:      StatusActive,
			Schedulable: true,
			Credentials: map[string]any{"access_token": "oauth-access-token"},
		},
	}}}

	manifest, err := svc.BuildLocalCodexModelsManifest(context.Background(), &groupID, "")
	require.ErrorIs(t, err, ErrNoAvailableAccounts)
	require.Nil(t, manifest)
}
