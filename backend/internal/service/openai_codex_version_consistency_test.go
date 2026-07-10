package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCodexVersionConstants_Consistency(t *testing.T) {
	require.Equal(t, codexCLIVersion, openAICodexProbeVersion)
	require.Contains(t, codexCLIUserAgent, "codex_cli_rs/"+codexCLIVersion)
	require.True(t, strings.Contains(codexCLIUserAgent, "xterm-256color"))
}
