//go:build integration

package repository

import (
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/suite"
)

type SessionLimitCacheSuite struct {
	IntegrationRedisSuite
	cache service.SessionLimitCache
}

func (s *SessionLimitCacheSuite) SetupTest() {
	s.IntegrationRedisSuite.SetupTest()
	s.cache = NewSessionLimitCache(s.rdb, 5)
}

func (s *SessionLimitCacheSuite) TestUnregisterSession_FreesSlotForNewSession_whenForwardFails() {
	// Given
	const accountID = int64(101)
	const maxSessions = 1
	idleTimeout := 5 * time.Minute

	allowed, err := s.cache.RegisterSession(s.ctx, accountID, "session-A", maxSessions, idleTimeout)
	s.RequireNoError(err, "RegisterSession A")
	s.True(allowed, "first session should be allowed")

	allowed, err = s.cache.RegisterSession(s.ctx, accountID, "session-B", maxSessions, idleTimeout)
	s.RequireNoError(err, "RegisterSession B before release")
	s.False(allowed, "new session should be rejected while A holds the only slot")

	// When
	s.RequireNoError(s.cache.UnregisterSession(s.ctx, accountID, "session-A"), "UnregisterSession A")

	// Then
	allowed, err = s.cache.RegisterSession(s.ctx, accountID, "session-B", maxSessions, idleTimeout)
	s.RequireNoError(err, "RegisterSession B after release")
	s.True(allowed, "new session should register immediately after release")

	count, err := s.cache.GetActiveSessionCount(s.ctx, accountID)
	s.RequireNoError(err, "GetActiveSessionCount")
	s.Equal(1, count, "only B should remain active")
}

func (s *SessionLimitCacheSuite) TestUnregisterSession_Idempotent_whenSessionMissingOrRepeated() {
	// Given
	const accountID = int64(102)

	// When / Then
	s.RequireNoError(s.cache.UnregisterSession(s.ctx, accountID, "missing"), "missing session release")
	s.RequireNoError(s.cache.UnregisterSession(s.ctx, accountID, "missing"), "repeat missing session release")
	s.RequireNoError(s.cache.UnregisterSession(s.ctx, accountID, ""), "empty session release")
}

func (s *SessionLimitCacheSuite) TestUnregisterSession_OnlyTargetsSpecifiedSession() {
	// Given
	const accountID = int64(103)
	const maxSessions = 2
	idleTimeout := 5 * time.Minute
	for _, sid := range []string{"session-A", "session-B"} {
		allowed, err := s.cache.RegisterSession(s.ctx, accountID, sid, maxSessions, idleTimeout)
		s.RequireNoError(err, "RegisterSession "+sid)
		s.True(allowed)
	}

	// When
	s.RequireNoError(s.cache.UnregisterSession(s.ctx, accountID, "session-A"), "UnregisterSession A")

	// Then
	active, err := s.cache.IsSessionActive(s.ctx, accountID, "session-A")
	s.RequireNoError(err, "IsSessionActive A")
	s.False(active, "A should be released")

	active, err = s.cache.IsSessionActive(s.ctx, accountID, "session-B")
	s.RequireNoError(err, "IsSessionActive B")
	s.True(active, "B should remain active")
}

func TestSessionLimitCacheSuite(t *testing.T) {
	suite.Run(t, new(SessionLimitCacheSuite))
}
