package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenAIModelAlias_GPT56BareAndPrefixedResolveToSol(t *testing.T) {
	bases := []string{
		"gpt-5.6",
		"gpt5.6",
		"gpt-5.6-none",
		"gpt-5.6-minimal",
		"gpt-5.6-low",
		"gpt-5.6-medium",
		"gpt-5.6-high",
		"gpt-5.6-xhigh",
		"gpt-5.6-max",
		"gpt-5.6-2026-07-09",
	}
	prefixes := []string{"", "openai/", "provider/"}

	for _, prefix := range prefixes {
		for _, base := range bases {
			input := prefix + base
			t.Run(input, func(t *testing.T) {
				require.Equal(t, "gpt-5.6-sol", normalizeCodexModel(input))
				require.True(t, isOpenAIOAuthServableModel(input))
			})
		}
	}
}

func TestOpenAIModelAlias_ConcreteGPT56FamiliesStayDistinct(t *testing.T) {
	families := []string{"gpt-5.6-sol", "gpt-5.6-terra", "gpt-5.6-luna"}
	prefixes := []string{"", "openai/", "provider/"}
	suffixes := []string{"", "-2026-07-09"}

	for _, prefix := range prefixes {
		for _, family := range families {
			for _, suffix := range suffixes {
				input := prefix + family + suffix
				t.Run(input, func(t *testing.T) {
					require.Equal(t, family, normalizeCodexModel(input))
					require.True(t, isOpenAIOAuthServableModel(input))
				})
			}
		}
	}
}

func TestOpenAIModelAlias_UnknownLookalikesStayUnsupported(t *testing.T) {
	inputs := []string{
		"gpt-5.6-preview",
		"gpt-5.6-ultra",
		"gpt-6-other",
		"provider/deepseek-v4-pro",
		"provider/",
		"provider/openai/gpt-5.6-preview",
	}

	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			_, ok := normalizeKnownCodexModel(input)
			require.False(t, ok)
			require.False(t, isOpenAIOAuthServableModel(input))
		})
	}

	_, ok := normalizeKnownCodexModel("")
	require.False(t, ok)
}

func TestOpenAIModelAlias_GPT6AstraAliasAndDefaultManifestAdvertiseOnce(t *testing.T) {
	require.Equal(t, "gpt-6-astra", normalizeCodexModel("gpt-6"))
	require.Equal(t, "gpt-6-astra", normalizeCodexModel("openai/gpt-6-astra"))
	require.True(t, isOpenAIOAuthServableModel("gpt-6"))
	require.True(t, isOpenAIOAuthServableModel("gpt-6-astra"))

	groupID := int64(11)
	svc := &OpenAIGatewayService{accountRepo: localCodexModelsAccountRepoStub{accounts: []Account{
		{
			ID:          620,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Credentials: map[string]any{"model_mapping": map[string]any{"gpt-*": "gpt-*"}},
		},
	}}}

	manifest, err := svc.BuildLocalCodexModelsManifest(context.Background(), &groupID, "")
	require.NoError(t, err)

	var envelope struct {
		Models []localCodexModel `json:"models"`
	}
	require.NoError(t, json.Unmarshal(manifest.Body, &envelope))
	counts := map[string]int{}
	for _, model := range envelope.Models {
		counts[model.Slug]++
	}
	require.Equal(t, 1, counts["gpt-6-astra"])
	require.Zero(t, counts["gpt-6"])
}

func TestOpenAIModelAlias_PreservesRequestedModelUsageFields(t *testing.T) {
	body := []byte(`{"model":"gpt-5.6","messages":[{"role":"user","content":"hi"}]}`)

	prepared, err := prepareOpenAIInputTokensCountRequest(body, &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth}, "")

	require.NoError(t, err)
	require.Equal(t, "gpt-5.6", prepared.OriginalModel)
	require.Equal(t, "gpt-5.6", prepared.NormalizedModel)
	require.Equal(t, "gpt-5.6-sol", prepared.UpstreamModel)
	require.Equal(t, "gpt-5.6-sol", prepared.Request.Model)
}

func TestOpenAIModelAlias_GPT6SolResolvesAndPreservesMax(t *testing.T) {
	for _, input := range []string{"gpt-6-sol", "openai/gpt-6-sol", "gpt-6-sol-max"} {
		t.Run(input, func(t *testing.T) {
			require.Equal(t, "gpt-6-sol", normalizeCodexModel(input))
			require.True(t, isOpenAIOAuthServableModel(input))
		})
	}
	require.Equal(t, "max", normalizeOpenAIReasoningEffortForModel("max", "gpt-6-sol"))
}
