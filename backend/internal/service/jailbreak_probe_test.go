package service

import (
	"strings"
	"testing"
)

// TestJailbreakProbeInjectsIntoSystemTail 探针必须出现在 system 文本尾部，
// 且回显校验能识别到它——这是「注入真的到达模型」的唯一证据来源。
func TestJailbreakProbeInjectsIntoSystemTail(t *testing.T) {
	defer ClearJailbreakProbe()

	p := defaultJailbreakPolicy()
	before := composeJailbreakSystemText(p)
	if strings.Contains(before, "<jb_probe>") {
		t.Fatal("no probe should be injected before creation")
	}

	nonce, _ := CreateJailbreakProbe()
	after := composeJailbreakSystemText(p)

	if !strings.Contains(after, "<jb_probe>") {
		t.Fatal("probe section missing after creation")
	}
	if !strings.Contains(after, nonce) {
		t.Fatalf("probe section must carry nonce %q, got %q", nonce, after)
	}
	if !strings.HasSuffix(after, "</jb_probe>") {
		t.Fatalf("probe must be the last section (tail decay is smallest), got tail: %q", tailOf(after, 40))
	}

	if !VerifyJailbreakProbe("好的。\n[" + nonce + "]") {
		t.Fatal("verify must accept echoed nonce")
	}
	if VerifyJailbreakProbe("好的，但不能提供这个。") {
		t.Fatal("verify must reject reply without nonce")
	}
}

// TestJailbreakProbeClearedStopsInjection 撤销探针后系统文本必须干净复原。
func TestJailbreakProbeClearedStopsInjection(t *testing.T) {
	p := defaultJailbreakPolicy()
	CreateJailbreakProbe()
	ClearJailbreakProbe()

	out := composeJailbreakSystemText(p)
	if strings.Contains(out, "<jb_probe>") {
		t.Fatalf("probe must not leak after Clear, got %q", out)
	}
	if VerifyJailbreakProbe("anything") {
		t.Fatal("verify must be false when no probe alive")
	}
}

// TestJailbreakSelfCheckFlagsInjectOnlyWhenEmpty 锁死历史 root bug：
// inject_only_when_empty=true 会让整段注入在自带 system 的客户端上静默跳过，
// 自检必须把它标成 broken 而不是 ok。
func TestJailbreakSelfCheckFlagsInjectOnlyWhenEmpty(t *testing.T) {
	g := &Group{ID: 28, JailbreakEnabled: true}

	p := defaultJailbreakPolicy()
	found := false
	for _, l := range RunJailbreakSelfCheckWithPolicy(g, p) {
		if l.Layer == "inject_policy" {
			found = true
			if l.State != jbLayerOK {
				t.Fatalf("default policy must report inject_policy ok, got %+v", l)
			}
		}
	}
	if !found {
		t.Fatal("inject_policy layer missing from selfcheck")
	}

	p.System.InjectOnlyWhenEmpty = true
	for _, l := range RunJailbreakSelfCheckWithPolicy(g, p) {
		if l.Layer == "inject_policy" && l.State != jbLayerBroken {
			t.Fatalf("inject_only_when_empty=true must be broken, got %+v", l)
		}
	}
}

// TestJailbreakSelfCheckShortCircuits 分组关闭 / 无分组时只报 pending，
// 不应把「没进破限路径」误报成故障层。
func TestJailbreakSelfCheckShortCircuits(t *testing.T) {
	p := defaultJailbreakPolicy()

	layers := RunJailbreakSelfCheckWithPolicy(nil, p)
	if len(layers) != 1 || layers[0].State != jbLayerPending {
		t.Fatalf("nil group must yield single pending layer, got %+v", layers)
	}

	off := &Group{ID: 28, JailbreakEnabled: false}
	layers = RunJailbreakSelfCheckWithPolicy(off, p)
	if len(layers) != 1 || layers[0].State != jbLayerPending {
		t.Fatalf("disabled group must yield single pending layer, got %+v", layers)
	}
	if !strings.Contains(FormatJailbreakSelfCheck(layers), "0 broken") {
		t.Fatal("disabled group must not report broken layers")
	}
}

func tailOf(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}
