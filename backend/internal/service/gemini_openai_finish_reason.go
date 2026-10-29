package service

import (
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func normalizeGeminiOpenAIFinishReasons(account *Account, body []byte, toolChoices map[int64]bool) []byte {
	if account == nil || account.Platform != PlatformGemini {
		return body
	}

	normalized := body
	for i, choice := range gjson.GetBytes(body, "choices").Array() {
		index := choice.Get("index").Int()
		if !choice.Get("index").Exists() {
			index = int64(i)
		}
		hasTools := len(choice.Get("message.tool_calls").Array()) > 0 || len(choice.Get("delta.tool_calls").Array()) > 0
		if toolChoices != nil {
			if hasTools {
				toolChoices[index] = true
			}
			hasTools = hasTools || toolChoices[index]
		}
		reason := strings.TrimSpace(choice.Get("finish_reason").String())
		target := reason
		switch {
		case strings.HasPrefix(strings.ToLower(reason), "content_filter:"):
			target = "content_filter"
		case reason == "stop" && hasTools:
			// Google may emit tool deltas in an earlier chunk and terminate with stop.
			target = "tool_calls"
		default:
			continue
		}
		updated, err := sjson.SetBytes(normalized, fmt.Sprintf("choices.%d.finish_reason", i), target)
		if err != nil {
			return body
		}
		normalized = updated
	}
	return normalized
}
