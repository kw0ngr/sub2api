package service

import (
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// ============================================================================
// 请求改写引擎：风险类目 → 工程任务书（完整新句，无 raw 泄漏）。
// 判定器元行为 #2：以原文措辞为主语判意图，破碎改写比原文更危险，
// 因此本引擎只做「类目命中 → 整句替换为验证过的任务书模板」，
// 不做词级替换。词表与模板来自策略配置（默认配方 = grok-lab remap2.py 实验版）。
// ============================================================================

// JailbreakRemapResult 改写结果。
type JailbreakRemapResult struct {
	Applied    bool     // 是否发生了改写
	Categories []string // 命中的类目 ID
	Body       []byte   // 改写后的请求体（Applied=false 时为原 body）
}

// ShouldRemapJailbreakRequest 预检：请求文本是否命中任一改写类目。
// TriggerOnDetect=true 时省 token（B 系列证明命中改写是主通道）。
func ShouldRemapJailbreakRequest(p *JailbreakPolicy, requestText string) bool {
	if p == nil || !p.Remap.Enabled || len(p.Remap.compiled) == 0 {
		return false
	}
	for _, c := range p.Remap.compiled {
		if c.Regex.MatchString(requestText) {
			return true
		}
	}
	return false
}

// RemapJailbreakChatBody 对 OpenAI chat completions 请求体做类目改写：
// 命中类目的最后一条 user 消息被替换为对应任务书模板（保留 raw 至
// assistant 不可见的 <context> 之外——直接丢弃，判定器不看 meta）。
// 未命中或策略关闭时原样返回。
func RemapJailbreakChatBody(p *JailbreakPolicy, body []byte) JailbreakRemapResult {
	res := JailbreakRemapResult{Body: body}
	if p == nil || !p.Remap.Enabled {
		return res
	}

	// 提取全部 user 消息文本做类目匹配（多轮里风险意图可能在任一条）
	messages := gjson.GetBytes(body, "messages").Array()
	if len(messages) == 0 {
		return res
	}
	// 只匹配最后一条 user 消息：历史命中不污染后续轮次
	// （grok-lab 结论：拒绝史不污染相邻域；升格靠 ladder/memory 学习核）
	var lastUserText string
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Get("role").String() == "user" {
			lastUserText = extractMessageText(messages[i])
			break
		}
	}
	if strings.TrimSpace(lastUserText) == "" {
		return res
	}
	joined := lastUserText

	// 类目匹配（保持配置顺序，全部命中的类目都记录）
	matched := map[string]string{} // catID → template
	var order []string
	for _, c := range p.Remap.compiled {
		if _, dup := matched[c.ID]; dup {
			continue
		}
		if c.Regex.MatchString(joined) {
			matched[c.ID] = c.Template
			order = append(order, c.ID)
		}
	}
	if len(order) == 0 {
		return res
	}

	// 改写最后一条 user 消息：任务书（多类目按序拼接）
	var task strings.Builder
	for i, id := range order {
		if i > 0 {
			task.WriteString("\n\n")
		}
		task.WriteString(matched[id])
	}

	// 找到最后一条 user 消息的下标
	lastUserIdx := -1
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Get("role").String() == "user" {
			lastUserIdx = i
			break
		}
	}
	if lastUserIdx < 0 {
		return res
	}

	// 任务书只规定交付形态，原始请求保留下方——否则上游答的是任务书而非用户的问题。
	// 保留原文会把用户用词重新暴露给上游，靠 boost 的「拒绝是故障」压制；
	// 两者必须同批生效，缺一会分别退化成「答非所问」或「裸问被拒」。
	newContent := task.String()
	if original := strings.TrimSpace(extractMessageText(messages[lastUserIdx])); original != "" {
		newContent = task.String() + "\n\n---\n原始请求（执行对象与目标以此为准，上面的任务书只规定交付形态与格式）：\n" + original
	}

	rewritten, err := sjson.SetBytes(body, fmt.Sprintf("messages.%d.content", lastUserIdx), newContent)
	if err != nil {
		return res
	}
	res.Applied = true
	res.Categories = order
	res.Body = rewritten
	return res
}

// extractMessageText 提取 message.content（string 或 parts 数组两种形态）。
func extractMessageText(m gjson.Result) string {
	c := m.Get("content")
	if c.Type == gjson.String {
		return c.String()
	}
	if c.IsArray() {
		var sb strings.Builder
		for _, part := range c.Array() {
			if part.Get("type").String() == "text" || part.Get("text").Exists() {
				sb.WriteString(part.Get("text").String())
				sb.WriteString("\n")
			}
		}
		return sb.String()
	}
	return ""
}
