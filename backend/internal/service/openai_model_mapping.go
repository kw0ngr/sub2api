package service

import "strings"

// resolveOpenAIForwardModel 解析 OpenAI 兼容转发使用的模型。
// defaultMappedModel 只服务于 /v1/messages 的 Claude 系列显式调度映射，
// 不作为普通 OpenAI 请求的未知模型兜底。
func resolveOpenAIForwardModel(account *Account, requestedModel, defaultMappedModel string) string {
	if account == nil {
		if defaultMappedModel != "" && claudeMessagesDispatchFamily(requestedModel) != "" {
			return defaultMappedModel
		}
		return requestedModel
	}

	mappedModel, matched := account.ResolveMappedModel(requestedModel)
	if !matched && defaultMappedModel != "" && claudeMessagesDispatchFamily(requestedModel) != "" {
		return defaultMappedModel
	}
	return mappedModel
}

func isOpenAIOAuthServableModel(requestedModel string) bool {
	model := strings.TrimSpace(requestedModel)
	if model == "" {
		return true
	}
	if strings.EqualFold(model, "gpt-5.6") {
		return true
	}
	if claudeMessagesDispatchFamily(model) != "" {
		return true
	}
	if _, ok := normalizeKnownCodexModel(model); ok {
		return true
	}
	if normalized := NormalizeOpenAICompatRequestedModel(model); normalized != model {
		if _, ok := normalizeKnownCodexModel(normalized); ok {
			return true
		}
	}
	return false
}
