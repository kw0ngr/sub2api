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

type geminiStreamToolIndices map[int64]*geminiChoiceToolIndices

type geminiChoiceToolIndices struct {
	byID map[string]int64
	next int64
}

func (indices geminiStreamToolIndices) normalize(body []byte) []byte {
	if indices == nil {
		return body
	}
	// Gemini 会跨 SSE 帧发送并行调用，每帧的数组下标都是 0，必须按调用 ID 分配 index。
	for i, choice := range gjson.GetBytes(body, "choices").Array() {
		choiceIndex := int64(i)
		if index := choice.Get("index"); index.Exists() {
			choiceIndex = index.Int()
		}
		for j, tool := range choice.Get("delta.tool_calls").Array() {
			id := tool.Get("id").String()
			state := indices[choiceIndex]
			if state == nil {
				state = &geminiChoiceToolIndices{byID: make(map[string]int64)}
				indices[choiceIndex] = state
			}
			index := tool.Get("index")
			if index.Exists() {
				if index.Int() >= state.next {
					state.next = index.Int() + 1
				}
				if id != "" {
					state.byID[id] = index.Int()
				}
				continue
			}
			if id == "" {
				continue
			}
			slot, known := state.byID[id]
			if !known {
				slot = state.next
				state.byID[id] = slot
				state.next++
			}
			if next, err := sjson.SetBytes(body, fmt.Sprintf("choices.%d.delta.tool_calls.%d.index", i, j), slot); err == nil {
				body = next
			}
		}
	}
	return body
}
