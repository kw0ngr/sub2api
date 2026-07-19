package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

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
