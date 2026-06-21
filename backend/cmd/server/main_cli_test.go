package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStartupCommand(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want startupCommandAction
	}{
		{
			name: "classifies version flag",
			args: []string{"--version"},
			want: startupCommandVersion,
		},
		{
			name: "classifies exact positional version",
			args: []string{"version"},
			want: startupCommandVersion,
		},
		{
			name: "keeps unknown positional on server path",
			args: []string{"not-version"},
			want: startupCommandServer,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given: command-line args for startup classification.
			args := tt.args

			// When: startup args are classified.
			got := startupCommand(args)

			// Then: only the version aliases take the version path.
			require.Equal(t, tt.want, got)
		})
	}
}

func TestVersionCommand(t *testing.T) {
	// Given: build metadata for the version command.
	version := "v-test"
	commit := "abc123"
	date := "2026-06-19"

	// When: the version line is formatted.
	got := versionCommand(version, commit, date)

	// Then: the existing version output format is preserved.
	require.Equal(t, "Sub2API v-test (commit: abc123, built: 2026-06-19)", got)
}
