package domain

import "testing"

func TestSupportedPlatformsIncludesAllGatewayPlatforms(t *testing.T) {
	t.Parallel()

	want := []string{
		PlatformAnthropic,
		PlatformOpenAI,
		PlatformGemini,
		PlatformAntigravity,
		PlatformOpenRouter,
		PlatformDeepSeek,
		PlatformGLM,
		PlatformGrok,
	}

	got := SupportedPlatforms()
	if len(got) != len(want) {
		t.Fatalf("SupportedPlatforms length = %d, want %d: %#v", len(got), len(want), got)
	}
	for i, platform := range want {
		if got[i] != platform {
			t.Fatalf("SupportedPlatforms[%d] = %q, want %q", i, got[i], platform)
		}
		if !IsSupportedPlatform(platform) {
			t.Fatalf("IsSupportedPlatform(%q) = false", platform)
		}
	}
	if IsSupportedPlatform("unknown") {
		t.Fatalf("unknown platform should not be supported")
	}
}

func TestDefaultAntigravityModelMapping_ImageCompatibilityAliases(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"gemini-2.5-flash-image":         "gemini-2.5-flash-image",
		"gemini-2.5-flash-image-preview": "gemini-2.5-flash-image",
		"gemini-3.1-flash-image":         "gemini-3.1-flash-image",
		"gemini-3.1-flash-image-preview": "gemini-3.1-flash-image",
		"gemini-3-pro-image":             "gemini-3.1-flash-image",
		"gemini-3-pro-image-preview":     "gemini-3.1-flash-image",
	}

	for from, want := range cases {
		got, ok := DefaultAntigravityModelMapping[from]
		if !ok {
			t.Fatalf("expected mapping for %q to exist", from)
		}
		if got != want {
			t.Fatalf("unexpected mapping for %q: got %q want %q", from, got, want)
		}
	}
}

func TestDefaultGrokModelMappingAliases(t *testing.T) {
	t.Parallel()

	if got := DefaultGrokModelMapping["grok"]; got != "grok-4.5" {
		t.Fatalf("DefaultGrokModelMapping[grok] = %q, want grok-4.5", got)
	}
	if got := DefaultGrokModelMapping["grok-4.6"]; got != "grok-4.6" {
		t.Fatalf("DefaultGrokModelMapping[grok-4.6] = %q, want identity mapping", got)
	}
	if got := DefaultGrokModelMapping["grok-4.20-reasoning"]; got != "grok-4.20-0309-reasoning" {
		t.Fatalf("DefaultGrokModelMapping[grok-4.20-reasoning] = %q", got)
	}
	if got := DefaultGrokModelMapping["grok-composer"]; got != "grok-composer-2.5-fast" {
		t.Fatalf("DefaultGrokModelMapping[grok-composer] = %q", got)
	}
}

func TestDefaultModelMappings_NewClaudeModels(t *testing.T) {
	t.Parallel()

	antigravityCases := map[string]string{
		"claude-fable-5":  "claude-fable-5",
		"claude-opus-4-8": "claude-opus-4-8",
	}
	for from, want := range antigravityCases {
		if got := DefaultAntigravityModelMapping[from]; got != want {
			t.Fatalf("DefaultAntigravityModelMapping[%s] = %q, want %q", from, got, want)
		}
	}

	bedrockCases := map[string]string{
		"claude-fable-5":  "anthropic.claude-fable-5",
		"claude-opus-4-8": "us.anthropic.claude-opus-4-8-v1",
		"claude-sonnet-5": "us.anthropic.claude-sonnet-5-v1",
	}
	for from, want := range bedrockCases {
		if got := DefaultBedrockModelMapping[from]; got != want {
			t.Fatalf("DefaultBedrockModelMapping[%s] = %q, want %q", from, got, want)
		}
	}
}
