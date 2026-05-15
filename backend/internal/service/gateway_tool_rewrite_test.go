package service

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestApplyToolNameRewriteToBodyRenamesToolUseInMessages(t *testing.T) {
	body := []byte(`{"tools":[{"name":"sessions_list","input_schema":{}},{"name":"web_search","type":"web_search_20250305"}],"tool_choice":{"type":"tool","name":"sessions_list"},"messages":[{"role":"assistant","content":[{"type":"tool_use","id":"tu_01","name":"sessions_list","input":{}},{"type":"text","text":"thinking"}]},{"role":"user","content":[{"type":"tool_result","tool_use_id":"tu_01","content":"ok"}]}]}`)
	rw := buildToolNameRewriteFromBody(body)
	require.NotNil(t, rw)
	require.Equal(t, "cc_sess_list", rw.Forward["sessions_list"])

	out := applyToolNameRewriteToBody(body, rw)

	require.Equal(t, "cc_sess_list", gjson.GetBytes(out, "tools.0.name").String())
	require.Equal(t, "web_search", gjson.GetBytes(out, "tools.1.name").String())
	require.Equal(t, "cc_sess_list", gjson.GetBytes(out, "tool_choice.name").String())
	require.Equal(t, "cc_sess_list", gjson.GetBytes(out, "messages.0.content.0.name").String())
	require.Equal(t, "thinking", gjson.GetBytes(out, "messages.0.content.1.text").String())
	require.False(t, gjson.GetBytes(out, "tools.1.cache_control").Exists(), "tool rewrite must not enable cache_control rewrite by default")
}

func TestRestoreToolNamesForContextRequiresStoredRewrite(t *testing.T) {
	body := []byte(`{"tools":[{"name":"sessions_list","input_schema":{}}]}`)
	rw := buildToolNameRewriteFromBody(body)
	require.NotNil(t, rw)

	restored := restoreToolNamesInBytes([]byte(`{"name":"cc_sess_list"}`), rw)
	require.JSONEq(t, `{"name":"sessions_list"}`, string(restored))

	unchanged := restoreToolNamesInBytes([]byte(`{"name":"cc_sess_list"}`), nil)
	require.JSONEq(t, `{"name":"cc_sess_list"}`, string(unchanged))
}
