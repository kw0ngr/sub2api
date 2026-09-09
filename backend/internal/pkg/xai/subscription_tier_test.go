package xai

import (
	"encoding/base64"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMapJWTSubscriptionTierNumber(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "tier 0 is free", raw: "0", want: "free"},
		{name: "tier 1 is supergrok", raw: "1", want: "supergrok"},
		{name: "tier 2 is x basic", raw: "2", want: "x_basic"},
		{name: "tier 3 is x premium", raw: "3", want: "x_premium"},
		{name: "tier 4 is x premium plus", raw: "4", want: "x_premium_plus"},
		{name: "tier 5 is supergrok heavy", raw: "5", want: "supergrok_heavy"},
		{name: "tier 6 is supergrok lite", raw: "6", want: "supergrok_lite"},
		{name: "tier 7 is supergrok plus", raw: "7", want: "supergrok_plus"},
		{name: "negative tier is empty", raw: "-1", want: ""},
		{name: "unknown tier is empty", raw: "8", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			headers := http.Header{}
			headers.Set("xai-subscription-tier", tt.raw)

			// When
			snapshot := ParseQuotaHeaders(headers, http.StatusOK)

			// Then
			require.NotNil(t, snapshot)
			require.Equal(t, tt.want, snapshot.SubscriptionTier)
		})
	}
}

func TestNormalizeSubscriptionTierAliases(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "free display", raw: " FREE ", want: "free"},
		{name: "free tier", raw: "free-tier", want: "free"},
		{name: "grok basic", raw: "grok_basic", want: "free"},
		{name: "supergrok", raw: "SuperGrok", want: "supergrok"},
		{name: "supergrok heavy spaced", raw: "SuperGrok Heavy", want: "supergrok_heavy"},
		{name: "supergrok pro alias", raw: "SuperGrokPro", want: "supergrok_heavy"},
		{name: "supergrok lite compact", raw: "SuperGrokLite", want: "supergrok_lite"},
		{name: "supergrok plus", raw: "supergrok-plus", want: "supergrok_plus"},
		{name: "x basic", raw: "X Basic", want: "x_basic"},
		{name: "x premium", raw: "x-premium", want: "x_premium"},
		{name: "x premium plus", raw: "x premium+", want: "x_premium_plus"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			headers := http.Header{}
			headers.Set("x-subscription-tier", tt.raw)

			// When
			snapshot := ParseQuotaHeaders(headers, http.StatusOK)

			// Then
			require.NotNil(t, snapshot)
			require.Equal(t, tt.want, snapshot.SubscriptionTier)
		})
	}
}

func TestSubscriptionTierFromJWTRejectsMalformedJWTMetadata(t *testing.T) {
	// Given
	validPayload := base64.RawURLEncoding.EncodeToString([]byte(`{"tier":7}`))
	garbagePayload := base64.RawURLEncoding.EncodeToString([]byte(`{"tier":7} trailing`))
	tests := []struct {
		name  string
		token string
		want  string
	}{
		{name: "forged three non-empty segments remain metadata only", token: "header." + validPayload + ".forged", want: "supergrok_plus"},
		{name: "trailing garbage json is empty", token: "header." + garbagePayload + ".forged", want: ""},
		{name: "missing signature segment is empty", token: "header." + validPayload, want: ""},
		{name: "empty signature segment is empty", token: "header." + validPayload + ".", want: ""},
		{name: "extra segment is empty", token: "header." + validPayload + ".forged.extra", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			got := SubscriptionTierFromJWT(tt.token)

			// Then
			require.Equal(t, tt.want, got)
		})
	}
}
