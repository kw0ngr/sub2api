package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeGitHubRepo(t *testing.T) {
	require.Equal(t, DefaultUpdateGitHubRepo, normalizeGitHubRepo(""))
	require.Equal(t, "kw0ngr/sub2api", normalizeGitHubRepo("kw0ngr/sub2api"))
	require.Equal(t, "kw0ngr/sub2api", normalizeGitHubRepo("https://github.com/kw0ngr/sub2api.git"))
	require.Equal(t, "kw0ngr/sub2api", normalizeGitHubRepo("github.com/kw0ngr/sub2api/releases"))
}

func TestValidateGitHubRepo(t *testing.T) {
	require.NoError(t, validateGitHubRepo("kw0ngr/sub2api"))
	require.Error(t, validateGitHubRepo(""))
	require.Error(t, validateGitHubRepo("only-owner"))
	require.Error(t, validateGitHubRepo("bad repo/name"))
	require.Error(t, validateGitHubRepo("a/b/c"))
}
