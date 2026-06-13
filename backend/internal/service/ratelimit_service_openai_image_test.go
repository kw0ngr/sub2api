package service

import (
	"net/http"
	"testing"
	"time"
)

func TestOpenAIImageRateLimitErrorDetection(t *testing.T) {
	body := []byte(`{"error":{"message":"Rate limit reached for limit gpt-image-2-codex in organization org; try again in 12s"}}`)
	if !isOpenAIImageRateLimitError(http.StatusTooManyRequests, body) {
		t.Fatal("expected OpenAI image 429 to be detected")
	}
	if isOpenAIImageRateLimitError(http.StatusBadRequest, body) {
		t.Fatal("did not expect non-429 image error to be treated as rate limit")
	}
	if isOpenAIImageRateLimitError(http.StatusTooManyRequests, []byte(`{"error":{"message":"text model rate limit"}}`)) {
		t.Fatal("did not expect text-only 429 to be treated as image rate limit")
	}
}

func TestParseOpenAIImageTryAgainCooldown(t *testing.T) {
	tests := []struct {
		body string
		want time.Duration
	}{
		{`try again in 250ms`, 250 * time.Millisecond},
		{`try again in 1.5 seconds`, 1500 * time.Millisecond},
		{`try again in 2 min`, 2 * time.Minute},
		{`no retry hint`, 0},
	}

	for _, tt := range tests {
		if got := parseOpenAIImageTryAgainCooldown([]byte(tt.body)); got != tt.want {
			t.Fatalf("parseOpenAIImageTryAgainCooldown(%q) = %s, want %s", tt.body, got, tt.want)
		}
	}
}
