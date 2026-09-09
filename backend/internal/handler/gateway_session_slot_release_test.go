package handler

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type releaseRecorder struct {
	accounts []*service.Account
}

func (r *releaseRecorder) release(account *service.Account) {
	r.accounts = append(r.accounts, account)
}

func TestSessionSlotReleaseTracker_ReleaseOne_releasesActualAccountAndRemovesIt(t *testing.T) {
	// Given
	tracker := newSessionSlotReleaseTracker()
	oldAccount := &service.Account{ID: 42, Name: "before-hydrate"}
	latestAccount := &service.Account{ID: 42, Name: "registered-account"}
	recorder := &releaseRecorder{}
	tracker.add(oldAccount)
	tracker.add(latestAccount)

	// When
	tracker.release(latestAccount, recorder.release)
	tracker.releaseAll(recorder.release)

	// Then
	require.Len(t, recorder.accounts, 1)
	require.Same(t, latestAccount, recorder.accounts[0])
}

func TestSessionSlotReleaseTracker_ReleaseOne_releasesUntrackedAccountForProfitVeto(t *testing.T) {
	// Given
	tracker := newSessionSlotReleaseTracker()
	account := &service.Account{ID: 7, Name: "profit-vetoed"}
	recorder := &releaseRecorder{}

	// When
	tracker.release(account, recorder.release)

	// Then
	require.Equal(t, []*service.Account{account}, recorder.accounts)
}

func TestSessionSlotReleaseTracker_ReleaseAll_releasesEveryTrackedAccountOnFinalFailure(t *testing.T) {
	// Given
	tracker := newSessionSlotReleaseTracker()
	first := &service.Account{ID: 1}
	second := &service.Account{ID: 2}
	recorder := &releaseRecorder{}
	tracker.add(first)
	tracker.add(second)

	// When
	tracker.releaseAll(recorder.release)
	tracker.releaseAll(recorder.release)

	// Then
	require.ElementsMatch(t, []*service.Account{first, second}, recorder.accounts)
}

func TestSessionSlotReleaseTracker_Retain_keepsSuccessAndPartialMeteredSessions(t *testing.T) {
	// Given
	tracker := newSessionSlotReleaseTracker()
	tracker.add(&service.Account{ID: 1})
	recorder := &releaseRecorder{}

	// When
	tracker.retain()
	tracker.releaseAll(recorder.release)

	// Then
	require.Empty(t, recorder.accounts)
}
