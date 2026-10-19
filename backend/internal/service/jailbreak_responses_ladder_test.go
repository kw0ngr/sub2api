package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/tidwall/gjson"
)

func TestAppendToLastResponsesUserInput(t *testing.T) {
	// string content 形态
	var rb map[string]any
	if err := json.Unmarshal([]byte(`{"input":[
		{"role":"user","content":"q1"},
		{"role":"assistant","content":"a1"},
		{"role":"user","content":"q2"}
	]}`), &rb); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out, ok := appendToLastResponsesUserInput(rb, "APPEND")
	if !ok {
		t.Fatal("must append to last user item")
	}
	raw, _ := json.Marshal(out)
	if got := gjson.GetBytes(raw, "input.2.content").String(); got != "q2\n\nAPPEND" {
		t.Fatalf("string content append, got %q", got)
	}

	// parts 数组形态：追加到最后一个 text part
	var rb2 map[string]any
	if err := json.Unmarshal([]byte(`{"input":[
		{"role":"user","content":[{"type":"input_text","text":"t1"}]}
	]}`), &rb2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out2, ok2 := appendToLastResponsesUserInput(rb2, "APPEND")
	if !ok2 {
		t.Fatal("must append to parts content")
	}
	raw2, _ := json.Marshal(out2)
	if got := gjson.GetBytes(raw2, "input.0.content.0.text").String(); got != "t1\n\nAPPEND" {
		t.Fatalf("parts append, got %q", got)
	}

	// 无 user 项：fail-open
	var rb3 map[string]any
	if err := json.Unmarshal([]byte(`{"input":[{"role":"assistant","content":"a"}]}`), &rb3); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok3 := appendToLastResponsesUserInput(rb3, "x"); ok3 {
		t.Fatal("no user item must return false")
	}

	// input 非 nil 但无 text part：补一个
	var rb4 map[string]any
	if err := json.Unmarshal([]byte(`{"input":[{"role":"user","content":42}]}`), &rb4); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// content 为数字时 switch 不命中任何 case，直接跳过——验证不 panic、返回 false
	if _, ok4 := appendToLastResponsesUserInput(rb4, "x"); ok4 {
		t.Fatal("non-string non-array content must return false (no panic)")
	}
}

func TestJbResponsesLadderRoundContext(t *testing.T) {
	ctx := context.Background()
	if jbResponsesLadderRound(ctx) != 0 {
		t.Fatal("default round must be 0")
	}
	ctx2 := withJbResponsesLadderRound(ctx, 2)
	if jbResponsesLadderRound(ctx2) != 2 {
		t.Fatal("round must round-trip via context")
	}
	if jbResponsesLadderRound(ctx) != 0 {
		t.Fatal("parent context must be unaffected")
	}
}
