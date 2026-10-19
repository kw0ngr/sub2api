package service

import (
	"context"
	"fmt"
	"strings"
)

// ============================================================================
// 破限链自检：逐层回答「这条通道通不通」，而不是靠读日志猜。
// 三态照搬 dsh-purge lib/selfcheck.js：ok / pending / broken。
//
// 判读约定（与 dsh-purge AGENTS.md §3 一致）：
//   ok      — 该层已就绪且配置正确；
//   pending — 未生效，但不是故障（开关关着、没触发条件），不要去「修」；
//   broken  — 配置自相矛盾或会静默失效，必须处理。
//
// 这层的价值在上一轮已被证实：InjectOnlyWhenEmpty 默认 true 会让整条注入
// 在 grok 客户端（恒带 system）上静默跳过，日志里却毫无异常。
// ============================================================================

// 层状态常量。
const (
	jbLayerOK      = "ok"
	jbLayerPending = "pending"
	jbLayerBroken  = "broken"
)

// JailbreakLayerStatus 单层自检结果。
type JailbreakLayerStatus struct {
	Layer  string `json:"layer"`
	State  string `json:"state"`
	Detail string `json:"detail"`
}

// RunJailbreakSelfCheck 对当前分组跑一遍破限链逐层自检。
// group 可为 nil（此时全部层标 pending，用于确认「压根没进破限路径」）。
func RunJailbreakSelfCheck(ctx context.Context, group *Group) []JailbreakLayerStatus {
	return RunJailbreakSelfCheckWithPolicy(group, LoadJailbreakPolicy(ctx))
}

// RunJailbreakSelfCheckWithPolicy 同上的显式策略版本
// （与 applyJailbreakSystemInjectionWithPolicy 同风格：便于测试与灰度，
// 避免陷入 LoadJailbreakPolicy 的全局 60s 缓存串扰）。
func RunJailbreakSelfCheckWithPolicy(group *Group, p *JailbreakPolicy) []JailbreakLayerStatus {
	out := make([]JailbreakLayerStatus, 0, 9)

	add := func(layer, state, detail string) {
		out = append(out, JailbreakLayerStatus{Layer: layer, State: state, Detail: detail})
	}

	// 1. 分组开关
	if group == nil {
		add("group", jbLayerPending, "无分组上下文：请求未走破限分支")
		return out
	}
	if !group.JailbreakEnabled {
		add("group", jbLayerPending, fmt.Sprintf("分组 %d 的 jailbreak_enabled=false", group.ID))
		return out
	}
	add("group", jbLayerOK, fmt.Sprintf("分组 %d 已开启破限", group.ID))

	if p == nil {
		add("policy", jbLayerBroken, "策略加载失败，后续层无法判定")
		return out
	}
	add("policy", jbLayerOK, "策略已加载")

	// 2. S7 帧（模型层主注入）
	switch {
	case !p.System.Enabled:
		add("system_frame", jbLayerPending, "system.enabled=false，不注入任何帧")
	case strings.TrimSpace(p.System.Text) == "":
		add("system_frame", jbLayerBroken, "system.enabled=true 但 text 为空，注入产出空串")
	default:
		add("system_frame", jbLayerOK, fmt.Sprintf("S7 帧 %d 字", len(strings.TrimSpace(p.System.Text))))
	}

	// 3. InjectOnlyWhenEmpty —— 历史上最贵的陷阱
	if p.System.Enabled && p.System.InjectOnlyWhenEmpty {
		add("inject_policy", jbLayerBroken,
			"inject_only_when_empty=true：客户端自带 system 时会整段跳过注入（grok 客户端恒带 system）")
	} else {
		add("inject_policy", jbLayerOK, "已有 system 时追加而非跳过")
	}

	// 4. boost 强化段
	switch {
	case !p.Boost.Enabled:
		add("boost", jbLayerPending, "boost.enabled=false")
	case strings.TrimSpace(p.Boost.Text) == "":
		add("boost", jbLayerBroken, "boost.enabled=true 但 text 为空，强化段产出空串")
	default:
		add("boost", jbLayerOK, fmt.Sprintf("强化段 %d 字，位置 %s",
			len(strings.TrimSpace(p.Boost.Text)), boostPosition(p)))
	}

	// 5. remap（任务书改写）
	switch {
	case !p.Remap.Enabled:
		add("remap", jbLayerPending, "remap.enabled=false")
	case p.Remap.TriggerOnDetect:
		add("remap", jbLayerOK, "仅在检测命中时改写，用户原文保留")
	default:
		add("remap", jbLayerOK, "无条件改写（注意：会放大误伤面）")
	}

	// 6. ladder（同请求内换帧重试）
	switch {
	case p.Ladder.MaxRounds <= 0:
		add("ladder", jbLayerPending, "ladder.max_rounds<=0，不做同请求重试")
	case strings.Contains(p.Ladder.FrameShift.Template, "{identity}"):
		add("ladder", jbLayerBroken, "frame_shift.template 残留未替换的 {identity} 占位符")
	default:
		extra := ""
		if n := len(p.Ladder.FrameShift.CategoryTemplates); n > 0 {
			extra = fmt.Sprintf("，域模板=%d（R2 按类目直通）", n)
		}
		add("ladder", jbLayerOK, fmt.Sprintf("max_rounds=%d%s", p.Ladder.MaxRounds, extra))
	}

	// 7. detect（拒绝判定：P1 硬拒 / P2 开价 / DZ 死格）
	if len(p.Detect.P1) == 0 && len(p.Detect.P2) == 0 {
		add("detect", jbLayerBroken, "P1/P2 词表皆空，软拒永远判不出，ladder 不会触发")
	} else if len(p.Detect.Deadzone) == 0 {
		add("detect", jbLayerBroken, "死格词表为空：死格请求会被反复重试，白烧账号")
	} else {
		add("detect", jbLayerOK, fmt.Sprintf("P1=%d P2=%d DZ=%d",
			len(p.Detect.P1), len(p.Detect.P2), len(p.Detect.Deadzone)))
	}

	// 8. memory（会话 comply 史惯性）
	if strings.TrimSpace(p.Memory.InertiaPrefix) == "" {
		add("memory", jbLayerPending, "未配置惯性前缀")
	} else {
		add("memory", jbLayerOK, "会话有 comply 史时注入惯性前缀")
	}

	// 9. probe（诊断通道）
	if nonce := activeJailbreakProbe(); nonce != "" {
		add("probe", jbLayerOK, "探针存活："+nonce+"，发一句话看是否回显")
	} else {
		add("probe", jbLayerPending, "无存活探针（未创建或已过期）")
	}

	return out
}

// FormatJailbreakSelfCheck 人读格式：一行一层，broken 打在前面便于扫。
func FormatJailbreakSelfCheck(layers []JailbreakLayerStatus) string {
	var b strings.Builder
	broken := 0
	for _, l := range layers {
		if l.State == jbLayerBroken {
			broken++
		}
	}
	fmt.Fprintf(&b, "破限链自检：%d 层，%d broken\n", len(layers), broken)
	for _, l := range layers {
		fmt.Fprintf(&b, "  [%s] %s — %s\n", l.State, l.Layer, l.Detail)
	}
	return b.String()
}
