package claude

import (
	"slices"
	"testing"
)

func TestDefaultModelIDsIncludesNewClaudeModels(t *testing.T) {
	t.Parallel()

	if !slices.Contains(DefaultModelIDs(), "claude-fable-5") {
		t.Fatalf("expected Claude default models to include claude-fable-5")
	}
	if !slices.Contains(DefaultModelIDs(), "claude-opus-4-8") {
		t.Fatalf("expected Claude default models to include claude-opus-4-8")
	}
	if !slices.Contains(DefaultModelIDs(), "claude-sonnet-5") {
		t.Fatalf("expected Claude default models to include claude-sonnet-5")
	}
}
