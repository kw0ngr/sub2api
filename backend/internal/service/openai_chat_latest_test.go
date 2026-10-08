package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChatLatestUsesInstantMetadataAndOwnPrice(t *testing.T) {
	for _, id := range []string{"chat-latest", "openai/chat-latest", "CHAT_LATEST"} {
		require.Equal(t, "chat-latest", normalizeKnownOpenAICodexModel(id))
		svc := &PricingService{}
		price := svc.GetModelPricing(id)
		require.NotNil(t, price)
		require.InDelta(t, 5e-6, price.InputCostPerToken, 1e-12)
		require.InDelta(t, .5e-6, price.CacheReadInputTokenCost, 1e-12)
		require.InDelta(t, 30e-6, price.OutputCostPerToken, 1e-12)
		require.Zero(t, price.LongContextInputTokenThreshold)
		require.False(t, price.SupportsServiceTier)
	}
	metadata, ok := localOfficialUpstreamModelMetadata("chat-latest")
	require.True(t, ok)
	require.NotNil(t, metadata.Reasoning)
	require.True(t, *metadata.Reasoning)
	require.Equal(t, 128000, metadata.MaxOutputTokens)

	model := buildLocalCodexModelFromSpec(localCodexModelSpec{
		Slug: "chat-latest", MetadataID: "chat-latest", ForceAPIKey: true,
	}, 1)
	require.Equal(t, "Chat Latest", model.DisplayName)
	require.Equal(t, 400000, model.ContextWindow)
	require.Equal(t, []string{"text", "image"}, model.InputModalities)
	require.Equal(t, localCodexReasoningLevels("medium"), model.SupportedReasoningLevels)
	require.True(t, model.SupportsReasoningSummaryParameter)
	require.False(t, model.UseResponsesLite)
	require.Empty(t, model.ServiceTiers)
	raw, err := json.Marshal(model)
	require.NoError(t, err)
	require.Contains(t, string(raw), `"service_tiers":[]`)
}

func TestRemoteCodexManifestUsesAccountRoutesAndArrayServiceTiers(t *testing.T) {
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{
		"model_mapping": map[string]any{"chat-latest": "chat-latest", "my-code": "gpt-6.1-sol"},
	}}
	body, err := projectRemoteCodexModelsManifest([]byte(`{
		"extra":"preserve",
		"models":[
			{"slug":"chat-latest","service_tiers":null},
			{"slug":"gpt-6.1-sol","context_window":1050000},
			{"slug":"blocked"}
		]
	}`), account)
	require.NoError(t, err)
	var envelope struct {
		Extra  string           `json:"extra"`
		Models []map[string]any `json:"models"`
	}
	require.NoError(t, json.Unmarshal(body, &envelope))
	require.Equal(t, "preserve", envelope.Extra)
	require.Len(t, envelope.Models, 2)
	require.Equal(t, "chat-latest", envelope.Models[0]["slug"])
	require.Equal(t, []any{}, envelope.Models[0]["service_tiers"])
	require.Equal(t, "my-code", envelope.Models[1]["slug"])
	require.Equal(t, float64(1050000), envelope.Models[1]["context_window"])
}

func TestChatLatestChecksEffectiveModelAndStripsUnsupportedSampling(t *testing.T) {
	require.ErrorContains(t, validateOpenAIModelReasoningCompatRequest(
		[]byte(`{"model":"my-chat","reasoning":{"effort":"max"}}`), "chat-latest"), "only supports")
	body, changed, err := normalizeOpenAIResponsesReasoningCompatibilityBody(
		[]byte(`{"model":"chat-latest","reasoning":{"effort":"medium"},"temperature":0.2,"top_p":0.9}`), "chat-latest")
	require.NoError(t, err)
	require.True(t, changed)
	require.NotContains(t, string(body), "temperature")
	require.NotContains(t, string(body), "top_p")
	require.Contains(t, string(body), `"effort":"medium"`)
}
