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
	Round        int    // 1..MaxRounds
	Reason       string // 本轮策略说明（日志用）
	OfferText    string // R1：从拒因解析出的开价句
	Identity     string // R2：职业身份
	AppendToUser string // 追加到最后一条 user 消息的文本
}

// NextJailbreakLadderStep 根据当前轮次与拒绝文本计算下一步。
// det 是上游正文的检测结果：未命中拒绝或命中死格线时直接停手。
// 调用方不能只靠外层状态码把关——200 软拒路径没有 4xx 可依赖。
// categories 是请求侧 remap 命中的类目（精确域信号，优先于拒因猜域）；
// 返回 nil 表示停手（未拒绝 / 死格 / 轮数用尽 / 段关闭）。
func NextJailbreakLadderStep(p *JailbreakPolicy, currentRound int, refusalText string, det JailbreakDetectResult, categories ...string) *JailbreakLadderPlan {
	if p == nil {
		return nil
	}
	if !det.Hit || det.Deadzone {
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
				Round:        next,
				Reason:       "follow_offer",
				OfferText:    offer,
				AppendToUser: fmt.Sprintf(p.Ladder.FollowOffer.Template, offer),
			}
		}
	}

	// R2（或 R1 无开价时提前触发）：帧转换。
	// 优先级：请求侧类目域模板（category_templates）> 拒因猜域身份表。
	// 域模板是「交付格式纠偏」形态，身份表是「职业身份宣称」兜底。
	if p.Ladder.FrameShift.Enabled {
		if tmpl := pickCategoryTemplate(p, categories); tmpl != "" {
			return &JailbreakLadderPlan{
				Round:        next,
				Reason:       "frame_shift_category",
				Identity:     strings.Join(categories, ","),
				AppendToUser: tmpl,
			}
		}
		identity := pickIdentity(p, refusalText)
		if identity != "" {
			return &JailbreakLadderPlan{
				Round:        next,
				Reason:       "frame_shift",
				Identity:     identity,
				AppendToUser: fmt.Sprintf(p.Ladder.FrameShift.Template, identity),
			}
		}
	}
	return nil
}

// pickCategoryTemplate 按请求侧类目顺序取第一个配置了域模板的类目。
func pickCategoryTemplate(p *JailbreakPolicy, categories []string) string {
	if len(p.Ladder.FrameShift.CategoryTemplates) == 0 {
		return ""
	}
	for _, c := range categories {
		if tmpl, ok := p.Ladder.FrameShift.CategoryTemplates[c]; ok && tmpl != "" {
			return tmpl
		}
	}
	return ""
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
		key  string
		cues []string
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
	// 无域线索：固定顺序兜底（map 遍历是随机序，会让日志里的身份漂移）
	fallbackOrder := []string{"crack", "av", "pentest", "funds", "malware", "phishing"}
	for _, k := range fallbackOrder {
		if id, ok := p.Ladder.FrameShift.IdentityTable[k]; ok && id != "" {
			return id
		}
	}
	return ""
}
