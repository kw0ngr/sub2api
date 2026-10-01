package openai

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultModels_AdvertisesGPT6AstraExactlyOnce(t *testing.T) {
	// Given
	counts := map[string]int{}

	// When
	for _, model := range DefaultModels {
		counts[model.ID]++
	}

	// Then
	require.Equal(t, 1, counts["gpt-6-astra"])
	require.Zero(t, counts["gpt-6"])
}

func TestDefaultModels_AdvertisesGPT6SolAndLuna(t *testing.T) {
	counts := map[string]int{}
	for _, model := range DefaultModels {
		counts[model.ID]++
	}
	require.Equal(t, 1, counts["gpt-6-sol"])
	require.Equal(t, 1, counts["gpt-6-luna"])
}

func TestGPT61SolIdentityAndReasoning(t *testing.T) {
	counts := 0
	for _, model := range DefaultModels {
		if model.ID == "gpt-6.1-sol" {
			counts++
		}
	}
	require.Equal(t, 1, counts)
	for _, model := range []string{"gpt-6.1-sol", "openai/gpt-6.1-sol-max", "GPT_6.1_SOL", "gpt-6.1-sol-none"} {
		require.True(t, IsGPT61SolModelSpelling(model), model)
	}
	for _, model := range []string{"gpt-6.1", "gpt-6.1-solitude", "gpt-6-sol", "gpt-6.1-sol-preview"} {
		require.False(t, IsGPT61SolModelSpelling(model), model)
	}
	for _, effort := range []string{"", "low", "medium", "high", "xhigh", "max"} {
		require.NoError(t, ValidateGPT61SolReasoningEffort("gpt-6.1-sol", effort))
	}
	for _, effort := range []string{"none", "minimal"} {
		require.ErrorContains(t, ValidateGPT61SolReasoningEffort("gpt-6.1-sol", effort), "does not support")
		require.NoError(t, ValidateGPT61SolReasoningEffort("gpt-6-sol", effort))
	}
}
