//go:build unit

package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDebugTraceStore_ListNewestFirstWithFilters(t *testing.T) {
	store := newDebugTraceStore(4, time.Hour)

	firstAccount := int64(1)
	secondAccount := int64(2)
	store.Add(&DebugTrace{RequestID: "req-1", Platform: PlatformOpenAI, StatusCode: 200, AccountID: &firstAccount, CreatedAt: time.Now().Add(-2 * time.Minute)})
	store.Add(&DebugTrace{RequestID: "req-2", Platform: PlatformOpenAI, StatusCode: 502, ReasonCode: "fallback_exhausted", FallbackTriggered: true, AccountID: &secondAccount, CreatedAt: time.Now().Add(-time.Minute)})
	store.Add(&DebugTrace{RequestID: "req-3", Platform: PlatformAnthropic, StatusCode: 429, ReasonCode: "upstream_rate_limited", FallbackTriggered: true, AccountID: &firstAccount, CreatedAt: time.Now()})

	items := store.List(DebugTraceFilter{Limit: 10, OnlyErrors: true})
	require.Len(t, items, 2)
	require.Equal(t, "req-3", items[0].RequestID)
	require.Equal(t, "req-2", items[1].RequestID)

	filtered := store.List(DebugTraceFilter{
		Limit:        10,
		OnlyErrors:   true,
		Platform:     PlatformOpenAI,
		OnlyFallback: true,
		ReasonCode:   "fallback_exhausted",
		AccountID:    &secondAccount,
	})
	require.Len(t, filtered, 1)
	require.Equal(t, "req-2", filtered[0].RequestID)

	got, ok := store.Get(items[0].ID)
	require.True(t, ok)
	require.Equal(t, "req-3", got.RequestID)
}
