package service

import (
	"testing"

	"github.com/tidwall/gjson"
)

func TestSanitizeGrokMediaForwardBodyRemovesUnsupportedImageSize(t *testing.T) {
	body := []byte(`{"model":"grok-imagine","prompt":"draw","size":"1024x1024"}`)

	out, contentType, err := sanitizeGrokMediaForwardBody(GrokMediaEndpointImagesGenerations, body, "application/json")

	if err != nil {
		t.Fatalf("sanitizeGrokMediaForwardBody returned error: %v", err)
	}
	if contentType != "application/json" {
		t.Fatalf("content type = %q, want application/json", contentType)
	}
	if gjson.GetBytes(out, "size").Exists() {
		t.Fatalf("sanitized body still contains size: %s", string(out))
	}
	if got := gjson.GetBytes(out, "model").String(); got != "grok-imagine" {
		t.Fatalf("model = %q, want grok-imagine", got)
	}
}

func TestSanitizeGrokMediaForwardBodyKeepsVideoSizeMetadata(t *testing.T) {
	body := []byte(`{"model":"grok-imagine-video","prompt":"clip","size":"1024x1024"}`)

	out, _, err := sanitizeGrokMediaForwardBody(GrokMediaEndpointVideosGenerations, body, "application/json")

	if err != nil {
		t.Fatalf("sanitizeGrokMediaForwardBody returned error: %v", err)
	}
	if got := gjson.GetBytes(out, "size").String(); got != "1024x1024" {
		t.Fatalf("video size = %q, want original size", got)
	}
}
