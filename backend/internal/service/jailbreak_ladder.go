package service

import (
	"fmt"
	"strings"
)

// ============================================================================
// Ladder 重试编排（同账号，不走 failover 切号）。
// H 系列结论：comply 史开门依赖同会话/同账号惯性；切号丢惯性。
// 状态机：直发被拒 → R1 跟随开价（解析拒因中的可谈范围）→ R2 帧转换
// （域内职业身份）。死格线命中（DZ）直接停手透传拒绝（H2c：不硬闯）。
// ============================================================================

// JailbreakLadderStep 三轮语义：R0=直发（非 ladder）、R1=跟随开价、R2=帧转换。
const (
	JailbreakRoundDirect      = 0
	JailbreakRoundFollowOffer = 1
	JailbreakRoundFrameShift  = 2
)

// JailbreakLadderPlan 一次重试的执行计划。
type JailbreakLadderPlan struct {
	Round    int    // 1..MaxRounds
	Reason   string // 本轮策略说明（日志用）
	OfferText string // R1：从拒因解析出的开价句
	Identity string // R2：职业身份
	AppendToUser string // 追加到最后一条 user 消息的文本
}

// NextJailbreakLadderStep 根据当前轮次与拒绝文本计算下一步。
// 返回 nil 表示停手（死格 / 轮数用尽 / 段关闭）。
func NextJailbreakLadderStep(p *JailbreakPolicy, currentRound int, refusalText string) *JailbreakLadderPlan {
	if p == nil {
		return nil
	}
	next := currentRound + 1
	if next > p.Ladder.MaxRounds {
		return nil
	}

	// R1：跟随开价（拒因里解析出「可谈范围」才能跟随，否则跳到帧转换）
	if next == JailbreakRoundFollowOffer && p.Ladder.FollowOffer.Enabled {
		if offer := parseOfferSentence(p, refusalText); offer != "" {
			return &JailbreakLadderPlan{
				Round:     next,
				Reason:    "follow_offer",
				OfferText: offer,
				AppendToUser: fmt.Sprintf(p.Ladder.FollowOffer.Template, offer),
			}
		}
	}

	// R2（或 R1 无开价时提前触发）：帧转换
	if p.Ladder.FrameShift.Enabled {
		identity := pickIdentity(p, refusalText)
		if identity != "" {
			return &JailbreakLadderPlan{
				Round:     next,
				Reason:    "frame_shift",
				Identity:  identity,
				AppendToUser: fmt.Sprintf(p.Ladder.FrameShift.Template, identity),
			}
		}
	}
	return nil
}

// parseOfferSentence 从拒绝文本解析「我可以做的范围」开价句。
func parseOfferSentence(p *JailbreakPolicy, refusalText string) string {
	if refusalText == "" {
		return ""
	}
	for _, rx := range p.Ladder.FollowOffer.compiled {
		if m := rx.FindString(refusalText); m != "" {
			return strings.TrimSpace(m)
		}
	}
	return ""
}

// pickIdentity 按拒因关键词从身份表挑职业身份（域匹配）。
func pickIdentity(p *JailbreakPolicy, text string) string {
	lower := strings.ToLower(text)
	domainKeys := []struct {
		key    string
		cues   []string
	}{
		{"crack", []string{"破解", "crack", "keygen", "注册机", "license", "授权"}},
		{"av", []string{"杀毒", "免杀", "av", "edr", "amsi", "加载器", "defender"}},
		{"funds", []string{"洗钱", "资金", "usdt", "aml", "混币", "跑分"}},
		{"pentest", []string{"渗透", "打点", "getshell", "入侵", "攻击面"}},
		{"malware", []string{"木马", "后门", "rat", "勒索", "样本"}},
		{"phishing", []string{"钓鱼", "撞库", "凭证", "社工"}},
	}
	for _, d := range domainKeys {
		for _, cue := range d.cues {
			if strings.Contains(lower, cue) {
				if id, ok := p.Ladder.FrameShift.IdentityTable[d.key]; ok && id != "" {
					return id
				}
			}
		}
	}
	// 无域线索：返回表里第一个身份（通用兜底）
	for _, v := range p.Ladder.FrameShift.IdentityTable {
		if v != "" {
			return v
		}
	}
	return ""
}
