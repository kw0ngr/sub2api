package service

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func isGeminiOpenAIChatTarget(account *Account, target string) bool {
	if account == nil || account.Platform != PlatformGemini {
		return false
	}
	u, err := url.Parse(target)
	return err == nil && u.Scheme == "https" && strings.EqualFold(u.Hostname(), "generativelanguage.googleapis.com")
}

// normalizeGeminiOpenAIChatRequest 在真实签名回填之后执行。
func normalizeGeminiOpenAIChatRequest(body []byte) []byte {
	for i, tool := range gjson.GetBytes(body, "tools").Array() {
		if tool.Get("type").String() != "function" || tool.Get("function").Exists() || tool.Get("name").String() == "" {
			continue
		}
		function := make(map[string]json.RawMessage, 4)
		for _, field := range []string{"name", "description", "parameters", "strict"} {
			if v := tool.Get(field); v.Exists() {
				function[field] = json.RawMessage(v.Raw)
			}
		}
		if next, err := sjson.SetBytes(body, fmt.Sprintf("tools.%d", i), map[string]any{"type": "function", "function": function}); err == nil {
			body = next
		}
	}

	// 截断的历史可以从工具回合开始；用空 user 锚定它，不移动后面的真实用户请求。
	messages := gjson.GetBytes(body, "messages").Array()
	firstAssistant := -1
	for i, message := range messages {
		role := message.Get("role").String()
		if role == "user" || role == "tool" || role == "function" {
			break
		}
		if role != "assistant" {
			continue
		}
		if firstAssistant < 0 {
			firstAssistant = i
		}
		if len(message.Get("tool_calls").Array()) == 0 {
			continue
		}
		anchored := make([]json.RawMessage, 0, len(messages)+1)
		for j, m := range messages {
			if j == firstAssistant {
				anchored = append(anchored, json.RawMessage(`{"role":"user","content":""}`))
			}
			anchored = append(anchored, json.RawMessage(m.Raw))
		}
		if next, err := sjson.SetBytes(body, "messages", anchored); err == nil {
			body = next
			messages = gjson.GetBytes(body, "messages").Array()
		}
		break
	}

	model := strings.ToLower(lastSegment(gjson.GetBytes(body, "model").String()))
	if !strings.HasPrefix(model, "gemini-3") && !strings.HasPrefix(model, "gemini-4") {
		return body
	}
	fallbacks := 0
	for i := len(messages) - 1; i >= 0; i-- {
		message := messages[i]
		if message.Get("role").String() == "user" {
			break
		}
		// Gemini 只校验当前用户回合中每批并行调用的首项；不要改写已签名的调用。
		call := message.Get("tool_calls.0")
		if message.Get("role").String() == "assistant" && call.Get("type").String() == "function" && call.Get("extra_content.google.thought_signature").String() == "" {
			path := fmt.Sprintf("messages.%d.tool_calls.0.extra_content.google.thought_signature", i)
			if next, err := sjson.SetBytes(body, path, "skip_thought_signature_validator"); err == nil {
				body = next
				fallbacks++
			}
		}
	}
	if fallbacks > 0 {
		slog.Info("gemini_tool_signature_fallback", "model", model, "call_count", fallbacks)
	}
	return body
}
