package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOfficialDeepSeekFlashAliasesResolveToV41APIModel(t *testing.T) {
	for _, alias := range []string{"deepseek-flash", "deepseek-v4.1-flash", "deepseek-v4-flash", "deepseek-v4-flash-vision-exp"} {
		for _, baseURL := range []string{"", "https://api.deepseek.com", "https://api.deepseek.com/v1"} {
			a := &Account{Platform: PlatformDeepSeek, Type: AccountTypeAPIKey,
				Credentials: map[string]any{"base_url": baseURL}}
			require.Equal(t, "deepseek-flash", a.GetMappedModel(alias), alias)
			a.Credentials["model_mapping"] = map[string]any{alias: alias}
			model, matched := a.ResolveMappedModel(alias)
			require.True(t, matched)
			require.Equal(t, "deepseek-flash", model)
		}
	}
	a := &Account{Platform: PlatformDeepSeek, Credentials: map[string]any{
		"model_mapping": map[string]any{"deepseek-flash": "deepseek-flash"},
	}}
	require.True(t, a.IsModelSupported("deepseek-v4.1-flash"))
	require.Equal(t, "deepseek-flash", a.GetMappedModel("deepseek-v4.1-flash"))
}

func TestDeepSeekFlashUpgradePreservesCustomProviderAndExplicitTargets(t *testing.T) {
	for _, platform := range []string{PlatformDeepSeek, PlatformOpenAI} {
		a := &Account{Platform: platform, Credentials: map[string]any{
			"base_url":      "https://custom.example/v1",
			"model_mapping": map[string]any{"deepseek-v4-flash": "deepseek-v4.1-flash"},
		}}
		require.Equal(t, "deepseek-v4.1-flash", a.GetMappedModel("deepseek-v4-flash"))
	}
	a := &Account{Platform: PlatformDeepSeek, Credentials: map[string]any{
		"model_mapping": map[string]any{"deepseek-v4-flash": "custom-flash"},
	}}
	require.Equal(t, "custom-flash", a.GetMappedModel("deepseek-v4-flash"))
	require.Equal(t, "deepseek-v4-pro", a.GetMappedModel("deepseek-v4-pro"))
}

func TestCustomDeepSeekOpenAIBaseDoesNotDuplicateAnthropicV1(t *testing.T) {
	a := &Account{Platform: PlatformDeepSeek, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"base_url": "https://custom.example/v1"}}
	require.Equal(t, "https://custom.example", anthropicCompatibleBaseURLForAccount(a))
	a.Credentials["base_url"] = "https://api.deepseek.com/v1"
	require.Equal(t, "https://api.deepseek.com/anthropic", anthropicCompatibleBaseURLForAccount(a))
}
