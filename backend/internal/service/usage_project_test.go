//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveUsageProjectIdentity_HashesStableKeyAndKeepsReadableLabel(t *testing.T) {
	key, label := ResolveUsageProjectIdentity("  acme-internal  ", "deployment-salt")

	require.Len(t, key, 16)
	require.NotContains(t, key, "acme")
	require.Equal(t, "acme-internal", label)

	keyAgain, labelAgain := ResolveUsageProjectIdentity("acme-internal", "deployment-salt")
	require.Equal(t, key, keyAgain)
	require.Equal(t, label, labelAgain)
}

func TestResolveUsageProjectIdentity_RejectsEmptyAfterTrim(t *testing.T) {
	key, label := ResolveUsageProjectIdentity("   ", "deployment-salt")

	require.Empty(t, key)
	require.Empty(t, label)
}
