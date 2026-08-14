//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSchedulerSnapshot_loadAccountsFromDB_retainsTransientlyCooledAccount(t *testing.T) {
	// Given
	groupID := int64(15)
	cooldownUntil := time.Now().Add(5 * time.Minute)
	account := Account{
		ID:                     1784,
		Platform:               PlatformGLM,
		Status:                 StatusActive,
		Schedulable:            true,
		TempUnschedulableUntil: &cooldownUntil,
		AccountGroups:          []AccountGroup{{GroupID: groupID}},
		GroupIDs:               []int64{groupID},
	}
	repo := &mockAccountRepoForPlatform{accounts: []Account{account}}
	snapshot := NewSchedulerSnapshotService(nil, nil, repo, nil, testConfig())

	// When
	accounts, err := snapshot.loadAccountsFromDB(context.Background(), SchedulerBucket{
		GroupID:  groupID,
		Platform: PlatformGLM,
		Mode:     SchedulerModeSingle,
	}, false)

	// Then
	require.NoError(t, err)
	require.Len(t, accounts, 1, "transient cooldown must not evict persistent bucket membership")
	require.False(t, accounts[0].IsSchedulable(), "request-time filtering must still honor the active cooldown")
}

func TestSchedulerSnapshot_ListSchedulableAccounts_recoversAtCooldownExpiryWithoutRebuild(t *testing.T) {
	// Given
	future := time.Now().Add(5 * time.Minute)
	account := &Account{ID: 1784, Status: StatusActive, Schedulable: true, TempUnschedulableUntil: &future}
	cache := &snapshotHydrationCache{snapshot: []*Account{account}}
	snapshot := NewSchedulerSnapshotService(cache, nil, nil, nil, testConfig())
	groupID := int64(15)

	// When: the cooldown is still active.
	accounts, _, err := snapshot.ListSchedulableAccounts(context.Background(), &groupID, PlatformGLM, false)

	// Then
	require.NoError(t, err)
	require.Empty(t, accounts)

	// When: time-based state expires without a bucket rebuild or outbox event.
	past := time.Now().Add(-time.Second)
	account.TempUnschedulableUntil = &past
	accounts, _, err = snapshot.ListSchedulableAccounts(context.Background(), &groupID, PlatformGLM, false)

	// Then
	require.NoError(t, err)
	require.Len(t, accounts, 1)
}
