package service

import (
	"strconv"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const deepSeekDisabledThinking = `{"type":"disabled"}`

func disableDeepSeekAnthropicThinkingForForcedToolChoice(body []byte) []byte {
	if !isDeepSeekForcedToolChoice(body) {
		return body
	}
	if gjson.GetBytes(body, "thinking.type").String() == "disabled" {
		return body
	}
	out, err := sjson.SetRawBytes(body, "thinking", []byte(deepSeekDisabledThinking))
	if err != nil {
		return body
	}
	return out
}

func isDeepSeekForcedToolChoice(body []byte) bool {
	toolChoice := gjson.GetBytes(body, "tool_choice")
	if !toolChoice.Exists() {
		return false
	}
	if strings.TrimSpace(gjson.GetBytes(body, "tool_choice.name").String()) != "" {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(gjson.GetBytes(body, "tool_choice.type").String())) {
	case "tool", "any":
		return true
	default:
		return false
	}
}

func repairDeepSeekAnthropicToolHistory(body []byte) []byte {
	switch strings.ToLower(strings.TrimSpace(gjson.GetBytes(body, "thinking.type").String())) {
	case "disabled":
		return body
	case "enabled", "adaptive":
	default:
		if strings.EqualFold(strings.TrimSpace(gjson.GetBytes(body, "output_config.effort").String()), "none") {
			return body
		}
	}
	if !gjson.ValidBytes(body) {
		return body
	}
	messages := gjson.GetBytes(body, "messages")
	if !messages.IsArray() {
		return body
	}
	updated := body
	for i, message := range messages.Array() {
		content := message.Get("content")
		if message.Get("role").Str != "assistant" || !content.IsArray() ||
			!content.Get(`#(type=="tool_use")`).Exists() ||
			content.Get(`#(type=="thinking")`).Exists() ||
			content.Get(`#(type=="redacted_thinking")`).Exists() {
			continue
		}
		// Official DeepSeek accepts a nonempty placeholder when no thinking history was recorded.
		raw := `[{"type":"thinking","thinking":" "},` + content.Raw[1:]
		next, err := sjson.SetRawBytes(updated, "messages."+strconv.Itoa(i)+".content", []byte(raw))
		if err != nil {
			return body
		}
		updated = next
	}
	return updated
}
