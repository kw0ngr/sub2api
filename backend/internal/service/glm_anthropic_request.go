package service

import (
	"bytes"
	"encoding/json"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func FilterGLMAnthropicUnsupportedBlocks(body []byte) []byte {
	return FilterRedactedThinkingBlocks(body)
}

func FilterRedactedThinkingBlocks(body []byte) []byte {
	if !bytes.Contains(body, []byte("redacted_thinking")) {
		return body
	}

	msgsRes := gjson.GetBytes(body, "messages")
	if !msgsRes.Exists() || !msgsRes.IsArray() {
		return body
	}

	var messages []any
	if err := json.Unmarshal(sliceRawFromBody(body, msgsRes), &messages); err != nil {
		return body
	}

	modified := false
	for _, msg := range messages {
		msgMap, ok := msg.(map[string]any)
		if !ok {
			continue
		}
		content, ok := msgMap["content"].([]any)
		if !ok {
			continue
		}

		role, _ := msgMap["role"].(string)
		var newContent []any
		changedThisMessage := false
		for i, block := range content {
			blockMap, ok := block.(map[string]any)
			if ok && blockMap["type"] == "redacted_thinking" {
				if newContent == nil {
					newContent = make([]any, 0, len(content))
					newContent = append(newContent, content[:i]...)
				}
				changedThisMessage = true
				continue
			}
			if newContent != nil {
				newContent = append(newContent, block)
			}
		}
		if !changedThisMessage {
			continue
		}
		modified = true
		if len(newContent) == 0 {
			placeholder := "(content removed)"
			if role == "assistant" {
				placeholder = "(assistant content removed)"
			}
			newContent = append(newContent, map[string]any{"type": "text", "text": placeholder})
		}
		msgMap["content"] = newContent
	}

	if !modified {
		return body
	}
	msgsBytes, err := json.Marshal(messages)
	if err != nil {
		return body
	}
	out, err := sjson.SetRawBytes(body, "messages", msgsBytes)
	if err != nil {
		return body
	}
	return out
}

func DisableGLMAnthropicThinkingByDefault(body []byte) []byte {
	if gjson.GetBytes(body, "thinking").Exists() {
		return body
	}
	out, err := sjson.SetBytes(body, "thinking.type", "disabled")
	if err != nil {
		return body
	}
	return out
}
