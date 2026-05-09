package tlsfingerprint

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultNode24ProfileExpandsDefaults(t *testing.T) {
	profile := DefaultNode24Profile()

	require.Equal(t, "Claude Code / Node.js 24.x", profile.Name)
	require.False(t, profile.EnableGREASE)
	require.Equal(t, defaultCipherSuites, profile.CipherSuites)
	require.Equal(t, []uint16{29, 23, 24}, profile.Curves)
	require.Equal(t, defaultPointFormats, profile.PointFormats)
	require.Equal(t, []string{"http/1.1"}, profile.ALPNProtocols)
	require.Equal(t, []uint16{0x0304, 0x0303}, profile.SupportedVersions)
	require.Equal(t, []uint16{29}, profile.KeyShareGroups)
	require.Equal(t, []uint16{1}, profile.PSKModes)
	require.Equal(t, defaultExtensionOrder, profile.Extensions)
}

func TestProfileCloneCopiesSlices(t *testing.T) {
	profile := DefaultNode24Profile()
	clone := profile.Clone()

	require.Equal(t, profile, clone)
	clone.CipherSuites[0] = 0
	clone.ALPNProtocols[0] = "h2"

	require.NotEqual(t, profile.CipherSuites[0], clone.CipherSuites[0])
	require.NotEqual(t, profile.ALPNProtocols[0], clone.ALPNProtocols[0])
}
