package service

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func normalizeOpenAICodexCompactReasoningEffortForAccount(c *gin.Context, account *Account, body []byte) ([]byte, bool, error) {
	if account == nil || !account.IsOpenAIOAuth() || !isOpenAIResponsesCompactPath(c) {
		return body, false, nil
	}

	requestedModel := strings.TrimSpace(gjson.GetBytes(body, "model").String())
	effectiveModel := account.GetMappedModel(requestedModel)
	return normalizeOpenAICodexCompactReasoningEffort(body, effectiveModel)
}

func normalizeOpenAICodexCompactReasoningEffort(body []byte, effectiveModel string) ([]byte, bool, error) {
	if !openAIModelSupportsMaxReasoning(effectiveModel) {
		return body, false, nil
	}

	rawEffort := strings.TrimSpace(gjson.GetBytes(body, "reasoning.effort").String())
	flatEffort := strings.TrimSpace(gjson.GetBytes(body, "reasoning_effort").String())
	if rawEffort == "" {
		rawEffort = flatEffort
	}
	if !strings.EqualFold(rawEffort, "max") {
		return body, false, nil
	}

	// Codex Ultra 会在客户端编排层为 GPT-5.6 下发 max；ChatGPT compact
	// 子端点当前只接受到 xhigh。只降级 OpenAI OAuth 的 /responses/compact，
	// 普通 Responses、API Key 请求和其他平台 OAuth 请求继续保留 max。
	normalized, err := sjson.SetBytes(body, "reasoning.effort", "xhigh")
	if err != nil {
		return body, false, fmt.Errorf("normalize codex compact reasoning effort: %w", err)
	}
	if flatEffort != "" && gjson.GetBytes(normalized, "reasoning_effort").Exists() {
		normalized, err = sjson.DeleteBytes(normalized, "reasoning_effort")
		if err != nil {
			return body, false, fmt.Errorf("delete codex compact flat reasoning effort: %w", err)
		}
	}
	return normalized, true, nil
}

func normalizeOpenAICodexCompactReasoningEffortInReqBody(c *gin.Context, account *Account, reqBody map[string]any, effectiveModel string) bool {
	if account == nil || !account.IsOpenAIOAuth() || !isOpenAIResponsesCompactPath(c) || reqBody == nil || !openAIModelSupportsMaxReasoning(effectiveModel) {
		return false
	}

	reasoning, _ := reqBody["reasoning"].(map[string]any)
	if reasoning != nil {
		if effort, ok := reasoning["effort"].(string); ok && strings.EqualFold(strings.TrimSpace(effort), "max") {
			reasoning["effort"] = "xhigh"
			return true
		}
	}

	if effort, ok := reqBody["reasoning_effort"].(string); ok && strings.EqualFold(strings.TrimSpace(effort), "max") {
		if reasoning == nil {
			reasoning = make(map[string]any)
			reqBody["reasoning"] = reasoning
		}
		reasoning["effort"] = "xhigh"
		delete(reqBody, "reasoning_effort")
		return true
	}

	return false
}
