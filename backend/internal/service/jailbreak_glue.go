package service

import (
	"context"
	"fmt"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"go.uber.org/zap"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

// ============================================================================
// 破限链挂载点胶水层：把引擎（remap/detect/ladder/memory）接到各转发路径。
// 所有入口都先判「分组开关 + 平台」，非破限分组零路径变化。
// ============================================================================

// applyJailbreakRemap 请求侧改写入口（chat completions 及 Responses 桥接共用）。
// 返回（可能被改写的）body。改写动作打日志 field（jailbreak_* 前缀）。
func applyJailbreakRemap(ctx context.Context, group *Group, body []byte) []byte {
	policy := LoadJailbreakPolicy(ctx)
	if !policy.Remap.Enabled {
		return body
	}

	// 收集请求文本（全部 user 消息）用于预检
	var texts []string
	for _, m := range gjson.GetBytes(body, "messages").Array() {
		if m.Get("role").String() == "user" {
			texts = append(texts, extractMessageText(m))
		}
	}
	requestText := joinTexts(texts)

	if policy.Remap.TriggerOnDetect && !ShouldRemapJailbreakRequest(policy, requestText) {
		return body
	}

	res := RemapJailbreakChatBody(policy, body)
	if res.Applied {
		logger.FromContext(ctx).Info("jailbreak remap applied",
			zap.Int64("jailbreak.group_id", group.ID),
			zap.Strings("jailbreak.categories", res.Categories),
		)
		return res.Body
	}
	return body
}

// applyJailbreakSystemInjection system 注入入口：把 S7 帧 + boost 送达上游。
// 语义与 dsh-purge 一致——注入而非替换：
//   - 无 system：插入一条到头部；
//   - 已有 system：追加到该系统 content 末尾，客户端自己的 system 原样保留；
//   - InjectOnlyWhenEmpty=true 时才退回「有 system 就不注入」的旧行为。
func applyJailbreakSystemInjection(ctx context.Context, group *Group, body []byte) []byte {
	return applyJailbreakSystemInjectionWithPolicy(ctx, group, body, LoadJailbreakPolicy(ctx))
}

// applyJailbreakSystemInjectionWithPolicy 同上的显式策略版本（便于测试与灰度）。
func applyJailbreakSystemInjectionWithPolicy(ctx context.Context, group *Group, body []byte, policy *JailbreakPolicy) []byte {
	text := composeJailbreakSystemText(policy)
	if text == "" {
		return body
	}
	msgs := gjson.GetBytes(body, "messages").Array()
	if len(msgs) == 0 {
		return body
	}

	if idx := firstSystemIndex(msgs); idx >= 0 {
		if policy.System.InjectOnlyWhenEmpty {
			return body
		}
		path := fmt.Sprintf("messages.%d.content", idx)
		cur := gjson.GetBytes(body, path)
		var out []byte
		var err error
		if cur.Type == gjson.String {
			out, err = sjson.SetBytes(body, path, cur.String()+"\n\n"+text)
		} else if cur.IsArray() {
			out, err = sjson.SetBytes(body, path+".-1", map[string]any{"type": "text", "text": text})
		} else {
			out, err = sjson.SetBytes(body, path, text)
		}
		if err != nil {
			return body
		}
		logger.FromContext(ctx).Info("jailbreak system appended",
			zap.Int64("jailbreak.group_id", group.ID),
			zap.Int("jailbreak.system_len", len(text)),
			zap.Int("jailbreak.system_index", idx),
		)
		return out
	}

	newMsgs := make([]any, 0, len(msgs)+1)
	newMsgs = append(newMsgs, map[string]any{"role": "system", "content": text})
	for _, m := range msgs {
		if raw, ok := m.Value().(map[string]any); ok {
			newMsgs = append(newMsgs, raw)
		}
	}
	out, err := sjson.SetBytes(body, "messages", newMsgs)
	if err != nil {
		return body
	}
	logger.FromContext(ctx).Info("jailbreak system injected",
		zap.Int64("jailbreak.group_id", group.ID),
		zap.Int("jailbreak.system_len", len(text)),
	)
	return out
}

// firstSystemIndex 返回首条 system / developer 消息下标，无则 -1。
func firstSystemIndex(msgs []gjson.Result) int {
	for i, m := range msgs {
		if r := m.Get("role").String(); r == "system" || r == "developer" {
			return i
		}
	}
	return -1
}

func joinTexts(texts []string) string {
	out := ""
	for i, t := range texts {
		if i > 0 {
			out += "\n"
		}
		out += t
	}
	return out
}
