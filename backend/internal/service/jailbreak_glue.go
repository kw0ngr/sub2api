package service

import (
	"context"

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

// applyJailbreakSystemInjection system 注入入口：messages 无 system 指令时
// 在数组头部插入策略 system.text。返回（可能注入后的）body。
func applyJailbreakSystemInjection(ctx context.Context, group *Group, body []byte) []byte {
	policy := LoadJailbreakPolicy(ctx)
	if !policy.System.Enabled || policy.System.Text == "" {
		return body
	}
	if policy.System.InjectOnlyWhenEmpty && hasAnySystemMessage(body) {
		return body
	}
	msgs := gjson.GetBytes(body, "messages").Array()
	if len(msgs) == 0 {
		return body
	}
	newMsgs := make([]any, 0, len(msgs)+1)
	newMsgs = append(newMsgs, map[string]any{"role": "system", "content": policy.System.Text})
	for _, m := range msgs {
		var v any
		if raw, ok := m.Value().(map[string]any); ok {
			v = raw
		} else {
			continue
		}
		newMsgs = append(newMsgs, v)
	}
	out, err := sjson.SetBytes(body, "messages", newMsgs)
	if err != nil {
		return body
	}
	logger.FromContext(ctx).Info("jailbreak system injected",
		zap.Int64("jailbreak.group_id", group.ID),
		zap.Int("jailbreak.system_len", len(policy.System.Text)),
	)
	return out
}

// hasAnySystemMessage 判断请求是否已带 system 指令。
func hasAnySystemMessage(body []byte) bool {
	for _, m := range gjson.GetBytes(body, "messages").Array() {
		if m.Get("role").String() == "system" || m.Get("role").String() == "developer" {
			return true
		}
	}
	return false
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
