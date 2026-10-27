package service

import (
	"encoding/json"
	"strings"
)

// Cloudflare 1010 is an edge-level request block, not an account health signal.
func isCloudflareBotBlockResponse(body []byte) bool {
	var payload any
	if json.Unmarshal(body, &payload) == nil {
		return containsCloudflareBotBlockCode(payload)
	}
	return strings.Contains(strings.ToLower(string(body)), "error code: 1010")
}

func containsCloudflareBotBlockCode(value any) bool {
	switch node := value.(type) {
	case map[string]any:
		cloudflareError := false
		for key, child := range node {
			if strings.EqualFold(key, "cloudflare_error") {
				cloudflareError, _ = child.(bool)
			}
		}
		for key, child := range node {
			if strings.EqualFold(key, "error_name") {
				if name, ok := child.(string); ok && strings.EqualFold(strings.TrimSpace(name), "browser_signature_banned") {
					return true
				}
			}
			if cloudflareError && (strings.EqualFold(key, "code") || strings.EqualFold(key, "error_code")) {
				switch code := child.(type) {
				case float64:
					if code == 1010 {
						return true
					}
				case string:
					if strings.TrimSpace(code) == "1010" {
						return true
					}
				}
			}
			if containsCloudflareBotBlockCode(child) {
				return true
			}
		}
	case []any:
		for _, child := range node {
			if containsCloudflareBotBlockCode(child) {
				return true
			}
		}
	}
	return false
}
