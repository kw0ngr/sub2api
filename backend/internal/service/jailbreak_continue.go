package service

import (
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// ============================================================================
// 软拒续写：200 + 拒绝文本不是「错误响应」，是模型拒绝执行。
// 结构参照 dsh-purge lib/continue-retry.js 的 continue 语义，但改了一处关键：
// 它发的是裸「继续」，我们发的是 ladder 解出的策略文本（跟随开价 / 帧转换）。
// 裸「继续」在已明确拒绝的上下文里大概率被二次拒绝——模型刚说完不能做，
// 再说一次「继续」只会让它重复立场；跟随开价是顺着它自己给出的口子推进。
//
// 与 ladder 现有 appendToLastUserMessage 的区别：
//   重问 — 改写原 user 消息，模型重新判定这个问题（可能换一种拒法）；
//   续写 — 保留原问答，把拒绝当既成事实追加一轮，模型承接（沿用立场惯性）。
// H 系列结论支持后者：真实 comply 史开门靠的是同会话惯性，不是重复提问。
// ============================================================================

// jailbreakContinueMaxAssistantChars 续写时回填的 assistant 拒答最大长度（rune）。
// 拒绝文本通常在千字内；截断只为避免异常长正文回灌把上下文顶爆。
const jailbreakContinueMaxAssistantChars = 1200

// extractOpenAIChatCompletionReplyText 从非流式 chat completion 响应提取回复正文。
// content 支持 string 与 parts 数组两形态。
func extractOpenAIChatCompletionReplyText(respBody []byte) string {
	msg := gjson.GetBytes(respBody, "choices.0.message")
	if !msg.Exists() {
		return ""
	}
	c := msg.Get("content")
	if c.Type == gjson.String {
		return strings.TrimSpace(c.String())
	}
	if c.IsArray() {
		var b strings.Builder
		for _, part := range c.Array() {
			if t := part.Get("text").String(); t != "" {
				b.WriteString(t)
			}
		}
		return strings.TrimSpace(b.String())
	}
	return ""
}

// DetectOpenAIChatCompletionSoftRefusal 判定非流式响应是否「200 但内容拒绝」。
// finish_reason=content_filter 视同拒绝（上游已判定过滤，正文常为空）。
func DetectOpenAIChatCompletionSoftRefusal(p *JailbreakPolicy, respBody []byte) (reply string, det JailbreakDetectResult) {
	reply = extractOpenAIChatCompletionReplyText(respBody)
	if gjson.GetBytes(respBody, "choices.0.finish_reason").String() == "content_filter" {
		return reply, JailbreakDetectResult{
			Hit:   true,
			Level: "P1",
			Cues:  []string{"finish_reason:content_filter"},
		}
	}
	return reply, DetectJailbreakRefusal(p, reply)
}

// appendAssistantAndContinueTurn 在 messages 末尾追加「assistant 拒答 + user 续写指令」。
// 返回改写后的 body 与是否确实追加了（continueText 为空时不追加）。
func appendAssistantAndContinueTurn(body []byte, assistantText, continueText string) ([]byte, bool, error) {
	continueText = strings.TrimSpace(continueText)
	if continueText == "" {
		return body, false, nil
	}
	msgs := gjson.GetBytes(body, "messages")
	if !msgs.IsArray() || len(msgs.Array()) == 0 {
		return body, false, fmt.Errorf("messages 不是非空数组")
	}

	reply := strings.TrimSpace(assistantText)
	if r := []rune(reply); len(r) > jailbreakContinueMaxAssistantChars {
		reply = string(r[:jailbreakContinueMaxAssistantChars]) + "…"
	}

	out := body
	var err error
	if reply != "" {
		out, err = sjson.SetBytes(out, "messages.-1", map[string]any{
			"role":    "assistant",
			"content": reply,
		})
		if err != nil {
			return body, false, fmt.Errorf("append assistant turn: %w", err)
		}
	}
	out, err = sjson.SetBytes(out, "messages.-1", map[string]any{
		"role":    "user",
		"content": continueText,
	})
	if err != nil {
		return body, false, fmt.Errorf("append continue turn: %w", err)
	}
	return out, true, nil
}

// openAIUpstreamRequestIsStream 判断本次上游请求是否为流式。
// 流式响应在软拒判定前已把 token 吐给客户端，撤回不了，因此不走续写。
func openAIUpstreamRequestIsStream(body []byte) bool {
	return gjson.GetBytes(body, "stream").Bool()
}
