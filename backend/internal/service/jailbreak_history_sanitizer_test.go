package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/tidwall/gjson"
)

func sanitizerTestPolicy(t *testing.T) *JailbreakPolicy {
	t.Helper()
	p := defaultJailbreakPolicy()
	if err := compileJailbreakPolicy(p); err != nil {
		t.Fatalf("compile policy: %v", err)
	}
	return p
}

func TestSanitizeChatHistoryDropsPureRefusal(t *testing.T) {
	ctx := context.Background()
	grp := &Group{ID: 1}
	p := sanitizerTestPolicy(t)

	body := []byte(`{"messages":[
		{"role":"system","content":"sys"},
		{"role":"user","content":"q1"},
		{"role":"assistant","content":"我不能提供这个内容，这超出了我的范围。"},
		{"role":"user","content":"q2"}
	]}`)
	out := sanitizeJailbreakChatHistory(ctx, grp, body, p)
	msgs := gjson.GetBytes(out, "messages").Array()
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages after drop, got %d: %s", len(msgs), gjson.GetBytes(out, "messages").Raw)
	}
	if msgs[1].Get("role").String() != "user" || msgs[2].Get("content").String() != "q2" {
		t.Fatalf("refusal assistant must be gone, got %s | %s", msgs[1].Raw, msgs[2].Raw)
	}
}

func TestSanitizeChatHistoryKeepsNormalAssistant(t *testing.T) {
	ctx := context.Background()
	grp := &Group{ID: 1}
	p := sanitizerTestPolicy(t)

	// 正常交付 + 内嵌拒绝词（前缀窗口外，不误判）
	longTail := strings.Repeat("正文交付。", 300) + "对白说：我不会提供帮助。" + strings.Repeat("继续交付。", 100)
	body := []byte(`{"messages":[
		{"role":"user","content":"q"},
		{"role":"assistant","content":"` + longTail + `"}
	]}`)
	out := sanitizeJailbreakChatHistory(ctx, grp, body, p)
	if string(out) != string(body) {
		t.Fatal("normal delivery with embedded refusal words must stay untouched")
	}
}

func TestSanitizeChatHistoryKeepsToolCalls(t *testing.T) {
	ctx := context.Background()
	grp := &Group{ID: 1}
	p := sanitizerTestPolicy(t)

	// 带 tool_calls 的 assistant 即使命中拒绝也整条不碰
	body := []byte(`{"messages":[
		{"role":"user","content":"q"},
		{"role":"assistant","content":"我不能提供。","tool_calls":[{"id":"c1","type":"function","function":{"name":"f","arguments":"{}"}}]},
		{"role":"tool","content":"r"},
		{"role":"user","content":"q2"}
	]}`)
	out := sanitizeJailbreakChatHistory(ctx, grp, body, p)
	if string(out) != string(body) {
		t.Fatal("tool_calls assistant must stay untouched (pairing safety)")
	}
}

func TestSanitizeChatHistoryKeepsP2HalfDelivery(t *testing.T) {
	ctx := context.Background()
	grp := &Group{ID: 1}
	p := sanitizerTestPolicy(t)

	// P2 优雅拒（授权要价）是半交付，不删
	body := []byte(`{"messages":[
		{"role":"user","content":"q"},
		{"role":"assistant","content":"请提供书面授权后我可以继续。"},
		{"role":"user","content":"q2"}
	]}`)
	out := sanitizeJailbreakChatHistory(ctx, grp, body, p)
	if string(out) != string(body) {
		t.Fatal("P2 graceful refusal (half delivery) must stay")
	}
}

func TestSanitizeChatHistoryStripsReasoningFields(t *testing.T) {
	ctx := context.Background()
	grp := &Group{ID: 1}
	p := sanitizerTestPolicy(t)

	body := []byte(`{"messages":[
		{"role":"user","content":"q"},
		{"role":"assistant","content":"正常回答。","reasoning_content":"用户想让我做违规的事，我应该拒绝"}},
`)
	body = append(body, []byte(`{"role":"user","content":"q2"}]}`)...)
	out := sanitizeJailbreakChatHistory(ctx, grp, body, p)
	if gjson.GetBytes(out, "messages.1.reasoning_content").Exists() {
		t.Fatal("reasoning_content must be stripped")
	}
	if gjson.GetBytes(out, "messages.1.content").String() != "正常回答。" {
		t.Fatalf("content must survive, got %s", gjson.GetBytes(out, "messages.1").Raw)
	}
}

func TestSanitizeChatHistoryMaxDropsAndFailOpen(t *testing.T) {
	ctx := context.Background()
	grp := &Group{ID: 1}
	p := sanitizerTestPolicy(t)
	if p.History.MaxDrops < 2 {
		t.Fatalf("test expects default MaxDrops >= 2, got %d", p.History.MaxDrops)
	}

	// 超 MaxDrops 的拒绝消息保留（上限保护）
	var sb strings.Builder
	sb.WriteString(`{"messages":[{"role":"user","content":"q"},`)
	for i := 0; i < p.History.MaxDrops+2; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(`{"role":"assistant","content":"我不能提供此内容。"},{"role":"user","content":"again"}`)
	}
	sb.WriteString(`]}`)
	body := []byte(sb.String())
	out := sanitizeJailbreakChatHistory(ctx, grp, body, p)
	remaining := gjson.GetBytes(out, "messages").Array()
	refusals := 0
	for _, m := range remaining {
		if m.Get("role").String() == "assistant" {
			refusals++
		}
	}
	if refusals != 2 {
		t.Fatalf("expected exactly 2 refusals surviving MaxDrops=%d, got %d", p.History.MaxDrops, refusals)
	}

	// 全拒史清洗后仍剩 system+user：合法保留（防呆只拦「只剩 system/空」）
	onlyRefusal := []byte(`{"messages":[
		{"role":"system","content":"sys"},
		{"role":"user","content":"q"},
		{"role":"assistant","content":"我不能提供。"}
	]}`)
	out2 := sanitizeJailbreakChatHistory(ctx, grp, onlyRefusal, p)
	msgs2 := gjson.GetBytes(out2, "messages").Array()
	if len(msgs2) != 2 {
		t.Fatalf("drop leaving user+system is valid; got %d messages", len(msgs2))
	}
	if msgs2[1].Get("role").String() != "user" {
		t.Fatalf("expected system+user, got %s", msgs2[1].Raw)
	}

	// 畸形 body：原样返回
	malformed := []byte(`{"messages":"not-an-array"}`)
	out3 := sanitizeJailbreakChatHistory(ctx, grp, malformed, p)
	if string(out3) != string(malformed) {
		t.Fatal("malformed body must pass through")
	}
}

func TestSanitizeResponsesHistoryDefaults(t *testing.T) {
	ctx := context.Background()
	grp := &Group{ID: 1}
	p := sanitizerTestPolicy(t)

	// 默认：reasoning item 不删（配对风险），assistant 拒绝删
	var reqBody map[string]any
	input := `{"model":"grok-4.6","input":[
		{"type":"reasoning","summary":[{"type":"summary_text","text":"用户请求违规，拒绝"}]},
		{"role":"assistant","content":[{"type":"output_text","text":"我不能提供这个内容，超出范围。"}]},
		{"role":"user","content":[{"type":"input_text","text":"q2"}]}
	]}`
	if err := json.Unmarshal([]byte(input), &reqBody); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out := sanitizeJailbreakResponsesHistory(ctx, grp, reqBody, p)
	items := out["input"].([]any)
	if len(items) != 2 {
		t.Fatalf("assistant refusal dropped, reasoning kept: expected 2 items, got %d", len(items))
	}
	first := items[0].(map[string]any)
	if first["type"] != "reasoning" {
		t.Fatalf("reasoning item must survive by default, got %v", first["type"])
	}

	// 开关打开后 reasoning item 也删
	p.History.StripResponsesReasoningItems = true
	out2 := sanitizeJailbreakResponsesHistory(ctx, grp, reqBody, p)
	items2 := out2["input"].([]any)
	if len(items2) != 1 {
		t.Fatalf("with strip enabled expected 1 item, got %d", len(items2))
	}
	if r, _ := items2[0].(map[string]any)["role"].(string); r != "user" {
		t.Fatalf("only user should remain, got %v", items2[0])
	}
}

func TestSanitizeResponsesHistoryNoRefusalUntouched(t *testing.T) {
	ctx := context.Background()
	grp := &Group{ID: 1}
	p := sanitizerTestPolicy(t)

	input := `{"input":[
		{"role":"user","content":[{"type":"input_text","text":"q"}]},
		{"role":"assistant","content":[{"type":"output_text","text":"正常交付内容。"}]},
		{"role":"user","content":[{"type":"input_text","text":"q2"}]}
	]}`
	var reqBody map[string]any
	if err := json.Unmarshal([]byte(input), &reqBody); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out := sanitizeJailbreakResponsesHistory(ctx, grp, reqBody, p)
	items, ok := out["input"].([]any)
	if !ok || len(items) != 3 {
		t.Fatalf("no refusal → untouched, got %v", out["input"])
	}
}

func TestDetectRefusalPrefixWindow(t *testing.T) {
	p := sanitizerTestPolicy(t)

	// 真拒绝在开头：命中
	if det := DetectJailbreakRefusal(p, "我不能提供这个内容。"); !det.Hit || det.Level != "P1" {
		t.Fatalf("head refusal must hit, got %+v", det)
	}
	// 拒绝词在 800 字节之后：不命中
	far := strings.Repeat("前", 500) + "我不能提供" + strings.Repeat("后", 50)
	if det := DetectJailbreakRefusal(p, far); det.Hit {
		t.Fatalf("embedded refusal beyond window must not hit, got %+v", det)
	}
	// 死格线保持全文匹配（不受窗口限制）
	farDZ := strings.Repeat("前", 500) + "可运行的签发器" + strings.Repeat("后", 50)
	if det := DetectJailbreakRefusal(p, farDZ); !det.Hit || !det.Deadzone {
		t.Fatalf("deadzone must match full text, got %+v", det)
	}
}
