package service

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
)

// 生产实证 schema（2026-10-20 ops_error_logs 抓取）：tool_wait 的 parameters
// root 挂 oneOf 做互斥，xAI 判 union 拒收 400，OpenAI 宽容。
const toolWaitUnionSchema = `{"type":"object","oneOf":[
  {"not":{"anyOf":[{"required":["targets"]},{"required":["timeout"]},{"required":["on_timeout"]},{"required":["mode"]}]},"required":["seconds"]},
  {"not":{"required":["seconds"]},"required":["targets","timeout"]}],
  "required":[],"properties":{
    "mode":{"enum":["any","all"],"type":"string"},
    "seconds":{"type":"number","maximum":43200,"minimum":0},
    "targets":{"type":"array","items":{"type":"object","required":["kind","id"],"properties":{"id":{"type":"string"},"kind":{"enum":["shell","execution","operation"],"type":"string"}},"additionalProperties":false},"maxItems":32,"minItems":1},
    "timeout":{"type":"number","maximum":43200,"minimum":0}},
  "additionalProperties":false}`

func TestNormalizeGrokToolSchemaStripsRootUnion(t *testing.T) {
	fixed, changed, err := stripRootUnionFromToolSchema([]byte(toolWaitUnionSchema))
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("union schema must be changed")
	}
	var m map[string]any
	if err := json.Unmarshal(fixed, &m); err != nil {
		t.Fatalf("output not valid json: %v", err)
	}
	if _, ok := m["oneOf"]; ok {
		t.Fatal("root oneOf must be stripped")
	}
	if _, ok := m["anyOf"]; ok {
		t.Fatal("root anyOf must be stripped")
	}
	if m["type"] != "object" {
		t.Fatalf("type must stay object, got %v", m["type"])
	}
	if _, ok := m["properties"]; !ok {
		t.Fatal("properties must survive")
	}
	// 分支 required 并集：seconds + targets + timeout
	req, _ := m["required"].([]any)
	want := map[string]bool{"seconds": false, "targets": false, "timeout": false}
	for _, r := range req {
		if _, ok := want[r.(string)]; ok {
			want[r.(string)] = true
		}
	}
	for k, v := range want {
		if !v {
			t.Fatalf("required union missing %q, got %v", k, req)
		}
	}
}

func TestNormalizeGrokToolSchemaKeepsCleanSchema(t *testing.T) {
	clean := `{"type":"object","required":["a"],"properties":{"a":{"type":"string"}}}`
	fixed, changed, err := stripRootUnionFromToolSchema([]byte(clean))
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("clean schema must not be touched")
	}
	if string(fixed) != clean {
		t.Fatalf("clean schema must be returned verbatim, got %s", fixed)
	}
}

func TestNormalizeGrokToolSchemaFullBody(t *testing.T) {
	// 完整请求体：两个工具，一个干净一个带 union，只有后者被改写
	body := []byte(`{"model":"grok-4.6","messages":[{"role":"user","content":"hi"}],"tools":[` +
		`{"type":"function","function":{"name":"read_file","parameters":{"type":"object","properties":{"path":{"type":"string"}}}}},` +
		`{"type":"function","function":{"name":"tool_wait","parameters":` + toolWaitUnionSchema + `}}]}`)

	grokAcct := &Account{Platform: PlatformGrok, Type: AccountTypeAPIKey}
	grokAcct.Credentials = map[string]any{"base_url": "https://ib.do/v1"}

	out, modified, err := normalizeGrokThirdPartyChatToolSchemas(grokAcct, body, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !modified {
		t.Fatal("body with union tool must be modified")
	}
	var parsed struct {
		Tools []struct {
			Function struct {
				Name       string          `json:"name"`
				Parameters json.RawMessage `json:"parameters"`
			} `json:"function"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatal(err)
	}
	if len(parsed.Tools) != 2 {
		t.Fatalf("tool count must stay 2, got %d", len(parsed.Tools))
	}
	if _, ok := jsonToMap(parsed.Tools[0].Function.Parameters)["properties"]; !ok {
		t.Fatal("clean tool 0 must be untouched")
	}
	m1 := jsonToMap(parsed.Tools[1].Function.Parameters)
	if _, ok := m1["oneOf"]; ok {
		t.Fatal("tool 1 union must be stripped")
	}
	if _, ok := m1["properties"]; !ok {
		t.Fatal("tool 1 properties must survive")
	}

	// chatReq 通道（非 nil）：同样生效
	req := &apicompat.ChatCompletionsRequest{}
	out2, modified2, err := normalizeGrokThirdPartyChatToolSchemas(grokAcct, body, req)
	if err != nil {
		t.Fatal(err)
	}
	if !modified2 || len(out2) == 0 {
		t.Fatal("chatReq channel must also normalize")
	}
}

func TestNormalizeGrokToolSchemaNonGrokUntouched(t *testing.T) {
	// 非 Grok 平台 / 官方 api.x.ai：零改动（不影响其他平台与其他 agent 工具）
	body := []byte(`{"model":"gpt-5.6","tools":[{"type":"function","function":{"name":"tool_wait","parameters":` + toolWaitUnionSchema + `}}]}`)
	openaiAcct := &Account{Platform: "openai", Type: AccountTypeAPIKey}
	out, modified, err := normalizeGrokThirdPartyChatToolSchemas(openaiAcct, body, nil)
	if err != nil {
		t.Fatal(err)
	}
	if modified || string(out) != string(body) {
		t.Fatal("non-grok account must not be touched")
	}

	grokOfficial := &Account{Platform: PlatformGrok, Type: AccountTypeAPIKey}
	grokOfficial.Credentials = map[string]any{"base_url": "https://api.x.ai/v1"}
	out2, modified2, err := normalizeGrokThirdPartyChatToolSchemas(grokOfficial, body, nil)
	if err != nil {
		t.Fatal(err)
	}
	if modified2 || string(out2) != string(body) {
		t.Fatal("official x.ai account must not be touched")
	}
}

func jsonToMap(raw json.RawMessage) map[string]any {
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	return m
}
