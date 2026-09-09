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
