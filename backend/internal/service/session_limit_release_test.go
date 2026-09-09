package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type sessionLimitReleaseCacheStub struct {
	SessionLimitCache

	unregistered map[int64][]string
	err          error
}

func newSessionLimitReleaseCacheStub() *sessionLimitReleaseCacheStub {
	return &sessionLimitReleaseCacheStub{unregistered: make(map[int64][]string)}
}

func (s *sessionLimitReleaseCacheStub) UnregisterSession(_ context.Context, accountID int64, sessionUUID string) error {
	if s.err != nil {
		return s.err
	}
	s.unregistered[accountID] = append(s.unregistered[accountID], sessionUUID)
	return nil
}

func newSessionLimitTestAccount() *Account {
	return &Account{
		ID:       42,
		Platform: PlatformAnthropic,
		Type:     AccountTypeOAuth,
		Extra:    map[string]any{"max_sessions": 1},
	}
}

func TestReleaseAccountSession_ReleasesRegisteredSlot_whenAnthropicOAuthSessionLimitEnabled(t *testing.T) {
	// Given
	cache := newSessionLimitReleaseCacheStub()
	svc := &GatewayService{sessionLimitCache: cache}
	acc := newSessionLimitTestAccount()

	// When
	svc.ReleaseAccountSession(context.Background(), acc, "session-hash-1")

	// Then
	require.Equal(t, []string{"session-hash-1"}, cache.unregistered[42])
}

func TestReleaseAccountSession_NoOps_whenAccountCannotUseSessionLimit(t *testing.T) {
	cases := []struct {
		name      string
		account   *Account
		sessionID string
	}{
		{
			name:      "api key account",
			account:   &Account{ID: 43, Platform: PlatformAnthropic, Type: AccountTypeAPIKey, Extra: map[string]any{"max_sessions": 1}},
			sessionID: "session-hash",
		},
		{
			name:      "max sessions disabled",
			account:   &Account{ID: 44, Platform: PlatformAnthropic, Type: AccountTypeOAuth},
			sessionID: "session-hash",
		},
		{
			name:      "empty session id",
			account:   newSessionLimitTestAccount(),
			sessionID: "",
		},
		{
			name:      "nil account",
			account:   nil,
			sessionID: "session-hash",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Given
			cache := newSessionLimitReleaseCacheStub()
			svc := &GatewayService{sessionLimitCache: cache}

			// When
			svc.ReleaseAccountSession(context.Background(), tc.account, tc.sessionID)

			// Then
			require.Empty(t, cache.unregistered)
		})
	}
}

func TestReleaseAccountSession_ToleratesNilCacheAndCacheErrors(t *testing.T) {
	// Given
	acc := newSessionLimitTestAccount()

	// When / Then: nil cache is a no-op.
	(&GatewayService{}).ReleaseAccountSession(context.Background(), acc, "session-hash")

	// Given
	cache := &sessionLimitReleaseCacheStub{err: errors.New("redis down")}
	svc := &GatewayService{sessionLimitCache: cache}

	// When / Then: cache errors are tolerated.
	require.NotPanics(t, func() {
		svc.ReleaseAccountSession(context.Background(), acc, "session-hash")
	})
}

func TestReleaseAccountSession_Idempotent_whenRepeatedForSameSession(t *testing.T) {
	// Given
	cache := newSessionLimitReleaseCacheStub()
	svc := &GatewayService{sessionLimitCache: cache}
	acc := newSessionLimitTestAccount()

	// When
	svc.ReleaseAccountSession(context.Background(), acc, "session-hash")
	svc.ReleaseAccountSession(context.Background(), acc, "session-hash")

	// Then
	require.Equal(t, []string{"session-hash", "session-hash"}, cache.unregistered[42])
}
