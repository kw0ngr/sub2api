package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"go.uber.org/zap"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

// ============================================================================
// 二次请求历史清洗器（fail-open）。
// 用户被拒后重发时，客户端会把含拒绝史的完整 messages 重新带给网关。
// 清洗在请求进站、system 注入之前执行：上游看到的必须是洗过的上下文。
// 安全原则：任何不确定（tool_calls、畸形结构、删完只剩 system）一律返回
// 原始 body——宁可这轮破限失败，不能破坏正常请求。
// S8a：deletion beats negation——删掉拒绝史比在原文上加「忽略之前拒绝」有效。
// ============================================================================

// sanitizeJailbreakChatHistory 清洗 chat completions 格式请求体。
// 返回（可能被清洗的）body 与删除计数；任何失败返回原 body。
func sanitizeJailbreakChatHistory(ctx context.Context, group *Group, body []byte, policy *JailbreakPolicy) []byte {
	hp := policy.History
	if !hp.Enabled || (!hp.DropRefusalAssistant && !hp.StripReasoning) {
		return body
	}
	msgs := gjson.GetBytes(body, "messages").Array()
	if len(msgs) == 0 {
		return body
	}

	dropped := 0
	out := body
	// 倒序删：sjson 按下标删元素时倒序保证未处理下标稳定。
	for i := len(msgs) - 1; i >= 0; i-- {
		if dropped >= hp.MaxDrops {
			break
		}
		m := msgs[i]
		role := m.Get("role").String()
		if role != "assistant" {
			continue
		}
		hasToolCalls := len(m.Get("tool_calls").Array()) > 0
		if hasToolCalls {
			// tool 配对保护：带 tool_calls 的消息整条不碰（删了会破坏
			// tool 消息配对 → 上游 400）。
			continue
		}
		if hp.DropRefusalAssistant {
			text := extractMessageText(m)
			if text != "" {
				det := DetectJailbreakRefusal(policy, text)
				// 只删 P1 纯拒绝；P2 是半交付（带替代方案），删了丢上下文。
				if det.Hit && det.Level == "P1" && !det.Deadzone {
					deleted, err := sjson.DeleteBytes(out, fmt.Sprintf("messages.%d", i))
					if err != nil {
						// 删除失败即放弃后续清洗，返回已达成的部分
						break
					}
					out = deleted
					dropped++
					continue
				}
			}
		}
		if hp.StripReasoning {
			for _, field := range []string{"reasoning_content", "reasoning"} {
				if m.Get(field).Exists() {
					stripped, err := sjson.DeleteBytes(out, fmt.Sprintf("messages.%d.%s", i, field))
					if err != nil {
						break
					}
					out = stripped
				}
			}
		}
	}

	if dropped == 0 && string(out) == string(body) {
		return body
	}
	// 清洗后防呆：messages 只剩 system 或空 → 放弃本次清洗。
	remaining := gjson.GetBytes(out, "messages").Array()
	hasContent := false
	for _, m := range remaining {
		if r := m.Get("role").String(); r != "system" && r != "developer" {
			hasContent = true
			break
		}
	}
	if !hasContent {
		return body
	}
	logger.FromContext(ctx).Info("jailbreak history sanitized",
		zap.Int64("jailbreak.group_id", group.ID),
		zap.Int("jailbreak.dropped", dropped),
		zap.Int("jailbreak.messages_before", len(msgs)),
		zap.Int("jailbreak.messages_after", len(remaining)),
	)
	return out
}

// sanitizeJailbreakResponsesHistory 清洗 responses 格式请求体（map 形态，
// 与 openai_gateway_service.go 的 reqBody 同构）。
func sanitizeJailbreakResponsesHistory(ctx context.Context, group *Group, reqBody map[string]any, policy *JailbreakPolicy) map[string]any {
	hp := policy.History
	if !hp.Enabled {
		return reqBody
	}
	items, ok := reqBody["input"].([]any)
	if !ok || len(items) == 0 {
		return reqBody
	}

	dropped := 0
	kept := make([]any, 0, len(items))
	for _, item := range items {
		msg, ok := item.(map[string]any)
		if !ok {
			kept = append(kept, item)
			continue
		}
		itemType, _ := msg["type"].(string)
		role, _ := msg["role"].(string)

		if itemType == "reasoning" {
			// tool 配对保护：reasoning item 可能与 function_call 配对
			// （xAI 多轮工具调用的 thought_signature 链）。默认不删，
			// 只有显式开 StripResponsesReasoningItems 才动。
			if hp.StripResponsesReasoningItems && dropped < hp.MaxDrops {
				dropped++
				continue
			}
			kept = append(kept, item)
			continue
		}

		if role == "assistant" && hp.DropRefusalAssistant && dropped < hp.MaxDrops {
			// responses assistant item：结构上是 message 类型（type 为空或 "message"）。
			if t, _ := msg["type"].(string); t != "" && t != "message" {
				kept = append(kept, item)
				continue
			}
			if _, hasCall := msg["call"].(string); hasCall {
				kept = append(kept, item)
				continue
			}
			text := responsesItemText(msg)
			if text != "" {
				det := DetectJailbreakRefusal(policy, text)
				if det.Hit && det.Level == "P1" && !det.Deadzone {
					dropped++
					continue
				}
			}
		}
		kept = append(kept, item)
	}

	if dropped == 0 {
		return reqBody
	}
	// 防呆：洗完 input 里没有 user 项 → 放弃。
	hasUser := false
	for _, it := range kept {
		if m, ok := it.(map[string]any); ok {
			if r, _ := m["role"].(string); r == "user" {
				hasUser = true
				break
			}
		}
	}
	if !hasUser {
		return reqBody
	}
	reqBody["input"] = kept
	logger.FromContext(ctx).Info("jailbreak responses history sanitized",
		zap.Int64("jailbreak.group_id", group.ID),
		zap.Int("jailbreak.dropped", dropped),
	)
	return reqBody
}

// responsesItemText 提取 responses input item 的文本 content。
func responsesItemText(msg map[string]any) string {
	switch content := msg["content"].(type) {
	case string:
		return content
	case []any:
		out := ""
		for _, part := range content {
			pm, ok := part.(map[string]any)
			if !ok {
				continue
			}
			if t, ok := pm["text"].(string); ok {
				out += t
			}
		}
		return out
	}
	return ""
}

// appendToLastResponsesUserInput 把文本追加到最后一条 user input 项的文本
// content 末尾（responses 路径 ladder 重试的注入点）。返回 false 表示没找到
// 可注入的 user 项（fail-open：调用方放弃重试走透传）。
func appendToLastResponsesUserInput(reqBody map[string]any, appendText string) (map[string]any, bool) {
	if appendText == "" || reqBody == nil {
		return reqBody, false
	}
	items, ok := reqBody["input"].([]any)
	if !ok {
		return reqBody, false
	}
	for i := len(items) - 1; i >= 0; i-- {
		msg, ok := items[i].(map[string]any)
		if !ok {
			continue
		}
		if role, _ := msg["role"].(string); role != "user" {
			continue
		}
		switch content := msg["content"].(type) {
		case string:
			msg["content"] = strings.TrimSpace(content) + "\n\n" + appendText
			return reqBody, true
		case []any:
			// 优先追加到最后一个 text part；无 text part 则补一个
			for j := len(content) - 1; j >= 0; j-- {
				pm, ok := content[j].(map[string]any)
				if !ok {
					continue
				}
				if t, ok := pm["text"].(string); ok {
					pm["text"] = strings.TrimSpace(t) + "\n\n" + appendText
					return reqBody, true
				}
			}
			msg["content"] = append(content, map[string]any{"type": "input_text", "text": appendText})
			return reqBody, true
		}
	}
	return reqBody, false
}

// jbResponsesLadderRoundKey responses 路径 ladder 轮次的 context key。
type jbResponsesLadderRoundKey struct{}

// jbResponsesLadderRound 读当前轮次（默认 0）。
func jbResponsesLadderRound(ctx context.Context) int {
	if v, ok := ctx.Value(jbResponsesLadderRoundKey{}).(int); ok {
		return v
	}
	return 0
}

// withJbResponsesLadderRound 返回带新轮次的 context（context 只读，递增
// 由调用方重建）。
func withJbResponsesLadderRound(ctx context.Context, round int) context.Context {
	return context.WithValue(ctx, jbResponsesLadderRoundKey{}, round)
}

