package claude

import (
	"slices"
	"testing"
)

func TestDefaultModelIDsIncludesOpus48(t *testing.T) {
	t.Parallel()

	if !slices.Contains(DefaultModelIDs(), "claude-opus-4-8") {
		t.Fatalf("expected Claude default models to include claude-opus-4-8")
	}
}
