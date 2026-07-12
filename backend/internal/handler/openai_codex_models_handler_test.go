package handler

import (
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestShouldFallbackCodexModelsToV1List(t *testing.T) {
	require.True(t, shouldFallbackCodexModelsToV1List("/v1/models", service.ErrNoAvailableAccounts))
	require.True(t, shouldFallbackCodexModelsToV1List("/backend-api/codex/models", service.ErrNoAvailableAccounts))
	require.False(t, shouldFallbackCodexModelsToV1List("/v1/models", errors.New("database down")))
}
