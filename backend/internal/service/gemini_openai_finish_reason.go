package service

import (
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func normalizeGeminiOpenAIFinishReasons(account *Account, body []byte) []byte {
	if account == nil || account.Platform != PlatformGemini {
		return body
	}

	normalized := body
	for i, choice := range gjson.GetBytes(body, "choices").Array() {
		reason := strings.TrimSpace(choice.Get("finish_reason").String())
		if !strings.HasPrefix(strings.ToLower(reason), "content_filter:") {
			continue
		}
		updated, err := sjson.SetBytes(normalized, fmt.Sprintf("choices.%d.finish_reason", i), "content_filter")
		if err != nil {
			return body
		}
		normalized = updated
	}
	return normalized
}
