package service

import (
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
