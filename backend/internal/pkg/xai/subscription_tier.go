package xai

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"strconv"
	"strings"
)

func MapJWTSubscriptionTier(tier uint64) string {
	switch tier {
	case 0:
		return "free"
	case 1:
		return "supergrok"
	case 2:
		return "x_basic"
	case 3:
		return "x_premium"
	case 4:
		return "x_premium_plus"
	case 5:
		return "supergrok_heavy"
	case 6:
		return "supergrok_lite"
	case 7:
		return "supergrok_plus"
	default:
		return ""
	}
}

func NormalizeSubscriptionTier(raw string) string {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if trimmed == "" || strings.HasPrefix(trimmed, "-") {
		return ""
	}
	if n, err := strconv.ParseUint(trimmed, 10, 64); err == nil {
		return MapJWTSubscriptionTier(n)
	}
	tier := strings.ReplaceAll(trimmed, "-", "_")
	tier = strings.Join(strings.Fields(tier), "_")
	switch tier {
	case "free", "grok_free", "grokfree", "free_tier", "freetier", "grok_basic", "grokbasic":
		return "free"
	case "supergrok", "grokpro":
		return "supergrok"
	case "supergrok_lite", "supergroklite":
		return "supergrok_lite"
	case "supergrok_heavy", "supergrokheavy", "supergrokpro":
		return "supergrok_heavy"
	case "supergrok_plus", "supergrokplus":
		return "supergrok_plus"
	case "x_basic", "xbasic", "basic":
		return "x_basic"
	case "x_premium", "xpremium":
		return "x_premium"
	case "x_premium_plus", "xpremiumplus", "x_premium+":
		return "x_premium_plus"
	default:
		return tier
	}
}

func SubscriptionTierFromJWT(token string) string {
	claims := decodeJWTClaims(token)
	if claims == nil {
		return ""
	}
	raw, ok := claims["tier"]
	if !ok || raw == nil {
		return ""
	}
	switch v := raw.(type) {
	case json.Number:
		n, err := v.Int64()
		if err != nil || n < 0 {
			return ""
		}
		return MapJWTSubscriptionTier(uint64(n))
	case float64:
		if v < 0 || v > 7 || v != float64(uint64(v)) {
			return ""
		}
		return MapJWTSubscriptionTier(uint64(v))
	case string:
		return knownJWTSubscriptionTier(v)
	default:
		return ""
	}
}

func decodeJWTClaims(token string) map[string]any {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil
	}
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			return nil
		}
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	claims := map[string]any{}
	if err := decoder.Decode(&claims); err != nil {
		return nil
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil
	}
	return claims
}

func knownJWTSubscriptionTier(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "-") {
		return ""
	}
	if n, err := strconv.ParseUint(trimmed, 10, 64); err == nil {
		return MapJWTSubscriptionTier(n)
	}
	tier := NormalizeSubscriptionTier(trimmed)
	if isKnownSubscriptionTier(tier) {
		return tier
	}
	return ""
}

func isKnownSubscriptionTier(tier string) bool {
	switch tier {
	case "free", "supergrok", "x_basic", "x_premium", "x_premium_plus", "supergrok_heavy", "supergrok_lite", "supergrok_plus":
		return true
	default:
		return false
	}
}
