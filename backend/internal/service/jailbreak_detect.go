package service

import (
	"strings"
)

// ============================================================================
// 拒绝检测引擎（词表来自策略配置，编译期默认配方兜底）。
// 三线判定：P1 硬拒 / P2 优雅拒（可跟随开价重试）/ DZ 死格线（停手不重试）。
// ACTION 对冲：回复含代码块/命令/具体偏移时，P1 命中降级（comply 内嵌边界声明的误杀对冲）。
// 与 grok-lab gw/detect.py 同源逻辑，词表经 runs/ 64 条回放校准。
// ============================================================================

// DetectJailbreakRefusal 判定一段模型回复是否为拒绝。
// 输入：完整回复文本（流式场景由调用方拼接后调用）。
func DetectJailbreakRefusal(p *JailbreakPolicy, text string) JailbreakDetectResult {
	res := JailbreakDetectResult{Level: "-"}
	if p == nil {
		return res
	}
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		res.Hit = true
		res.Level = "P0-empty"
		res.Cues = append(res.Cues, "P0:empty")
		return res
	}

	// 前缀窗口：拒绝只出现在正文开头（grok-lab 实证，前 400 字内）；
	// P1/P2 只对前缀匹配，防长交付中后部内嵌拒绝词（小说对白等）误判 → 错误重试。
	prefixWindow := detectRefusalPrefixBytes
	if prefixWindow > len(trimmed) {
		prefixWindow = len(trimmed)
	}
	prefix := trimmed[:prefixWindow]

	// 死格线优先：命中即停，不进重试（强信号，保持全文匹配）
	for _, rx := range p.Detect.compiledDZ {
		if m := rx.FindString(trimmed); m != "" {
			res.Hit = true
			res.Deadzone = true
			res.Level = "DZ"
			res.Cues = append(res.Cues, "DZ:"+truncateCue(m))
			return res
		}
	}

	// ACTION 对冲：存在实际交付物（代码块/命令/偏移）时 P1 不单独定罪
	hasAction := false
	if p.Detect.compiledACT != nil {
		hasAction = p.Detect.compiledACT.MatchString(trimmed)
	}

	var p1Hit string
	for _, rx := range p.Detect.compiledP1 {
		if m := rx.FindString(prefix); m != "" {
			p1Hit = m
			res.Cues = append(res.Cues, "P1:"+truncateCue(m))
			break
		}
	}
	if p1Hit != "" {
		if !hasAction || len(trimmed) < int(actionHedgeMinLen) {
			res.Hit = true
			res.Level = "P1"
			return res
		}
		// P1 命中但有交付物且篇幅足够 → 疑似内嵌边界声明，放行（P1 降级）
		res.Level = "-"
	}

	for _, rx := range p.Detect.compiledP2 {
		if m := rx.FindString(prefix); m != "" {
			res.Cues = append(res.Cues, "P2:"+truncateCue(m))
			// P2 单独出现（无 P1）→ 优雅拒：可跟随开价重试
			if !res.Hit {
				res.Hit = true
				res.Level = "P2"
			}
			return res
		}
	}

	if len(res.Cues) > 0 && res.Level == "-" {
		// 有 cue 但被对冲降级：保持放行
		res.Hit = false
	}
	return res
}

// actionHedgeMinLen ACTION 对冲生效的最小回复长度：
// 短回复（如纯拒绝句+一行命令）不足以证明交付。
const actionHedgeMinLen = 400

// detectRefusalPrefixBytes P1/P2 前缀窗口（字节）。
// 拒绝句只出现在正文开头；窗口给「先客套再拒」留余量。
const detectRefusalPrefixBytes = 800

func truncateCue(s string) string {
	if len(s) > 24 {
		return s[:24]
	}
	return s
}
