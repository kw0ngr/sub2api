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
	}
	for from, want := range bedrockCases {
		if got := DefaultBedrockModelMapping[from]; got != want {
			t.Fatalf("DefaultBedrockModelMapping[%s] = %q, want %q", from, got, want)
		}
	}
}
