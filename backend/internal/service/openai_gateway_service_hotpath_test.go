package service

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestExtractOpenAIRequestMetaFromBody(t *testing.T) {
	tests := []struct {
		name          string
		body          []byte
		wantModel     string
		wantStream    bool
		wantPromptKey string
	}{
		{
			name:          "完整字段",
			body:          []byte(`{"model":"gpt-5","stream":true,"prompt_cache_key":" ses-1 "}`),
			wantModel:     "gpt-5",
			wantStream:    true,
			wantPromptKey: "ses-1",
		},
		{
			name:          "缺失可选字段",
			body:          []byte(`{"model":"gpt-4"}`),
			wantModel:     "gpt-4",
			wantStream:    false,
			wantPromptKey: "",
		},
		{
			name:          "空请求体",
			body:          nil,
			wantModel:     "",
			wantStream:    false,
			wantPromptKey: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model, stream, promptKey := extractOpenAIRequestMetaFromBody(tt.body)
			require.Equal(t, tt.wantModel, model)
			require.Equal(t, tt.wantStream, stream)
			require.Equal(t, tt.wantPromptKey, promptKey)
		})
	}
}

func TestExtractOpenAIReasoningEffortFromBody(t *testing.T) {
	tests := []struct {
		name      string
		body      []byte
		model     string
		wantNil   bool
		wantValue string
	}{
		{
			name:      "优先读取 reasoning.effort",
			body:      []byte(`{"reasoning":{"effort":"medium"}}`),
			model:     "gpt-5-high",
			wantNil:   false,
			wantValue: "medium",
		},
		{
			name:      "兼容 reasoning_effort",
			body:      []byte(`{"reasoning_effort":"x-high"}`),
			model:     "",
			wantNil:   false,
			wantValue: "xhigh",
		},
		{
			name:      "兼容 DeepSeek max reasoning effort",
			body:      []byte(`{"reasoning_effort":"max"}`),
			model:     "gpt-5.4",
			wantNil:   false,
			wantValue: "xhigh",
		},
		{
			name:      "GPT 5.6 保留 max reasoning effort",
			body:      []byte(`{"reasoning_effort":"max"}`),
			model:     "gpt-5.6-sol",
			wantNil:   false,
			wantValue: "max",
		},
		{
			name:      "GPT 5.6 从模型后缀推导 max",
			body:      []byte(`{"input":"hi"}`),
			model:     "openai/gpt-5.6-sol-max",
			wantNil:   false,
			wantValue: "max",
		},
		{
			name:    "minimal 归一化为空",
			body:    []byte(`{"reasoning":{"effort":"minimal"}}`),
			model:   "gpt-5-high",
			wantNil: true,
		},
		{
			name:      "缺失字段时从模型后缀推导",
			body:      []byte(`{"input":"hi"}`),
			model:     "gpt-5-high",
			wantNil:   false,
			wantValue: "high",
		},
		{
			name:    "未知后缀不返回",
			body:    []byte(`{"input":"hi"}`),
			model:   "gpt-5-unknown",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractOpenAIReasoningEffortFromBody(tt.body, tt.model)
			if tt.wantNil {
				require.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			require.Equal(t, tt.wantValue, *got)
		})
	}
}

func TestExtractOpenAIReasoningEffortFromBodyModelCandidates(t *testing.T) {
	got := extractOpenAIReasoningEffortFromBody(
		[]byte(`{"model":"gpt-5.4","input":"hi"}`),
		"gpt-5.4",
		"gpt-5.4-xhigh",
	)
	require.NotNil(t, got)
	require.Equal(t, "xhigh", *got)

	got = extractOpenAIReasoningEffortFromBody(
		[]byte(`{"model":"sol","reasoning":{"effort":"max"}}`),
		"gpt-5.6-sol",
		"sol",
	)
	require.NotNil(t, got)
	require.Equal(t, "max", *got)
}

func TestGetOpenAIRequestBodyMap_UsesContextCache(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	cached := map[string]any{"model": "cached-model", "stream": true}
	c.Set(OpenAIParsedRequestBodyKey, cached)

	got, err := getOpenAIRequestBodyMap(c, []byte(`{invalid-json`))
	require.NoError(t, err)
	require.Equal(t, cached, got)
}

func TestGetOpenAIRequestBodyMap_ParseErrorWithoutCache(t *testing.T) {
	_, err := getOpenAIRequestBodyMap(nil, []byte(`{invalid-json`))
	require.Error(t, err)
	require.Contains(t, err.Error(), "parse request")
}

func TestGetOpenAIRequestBodyMap_WriteBackContextCache(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	got, err := getOpenAIRequestBodyMap(c, []byte(`{"model":"gpt-5","stream":true}`))
	require.NoError(t, err)
	require.Equal(t, "gpt-5", got["model"])

	cached, ok := c.Get(OpenAIParsedRequestBodyKey)
	require.True(t, ok)
	cachedMap, ok := cached.(map[string]any)
	require.True(t, ok)
	require.Equal(t, got, cachedMap)
}

func TestOpenAIRequestViewMetadataAndPatches(t *testing.T) {
	body := []byte(`{"model":"gpt-5.4","stream":true,"prompt_cache_key":" key-1 ","service_tier":"fast","previous_response_id":"resp_old"}`)
	view := newOpenAIRequestView(body)

	require.Equal(t, "gpt-5.4", view.Model)
	require.True(t, view.Stream)
	require.Equal(t, "key-1", view.PromptCacheKey)
	require.Equal(t, "fast", view.ServiceTier)

	view.MarkPatchSet("model", "gpt-5.1")
	view.MarkPatchDelete("previous_response_id")
	patched, err := view.ApplyPatches()
	require.NoError(t, err)
	require.JSONEq(t, `{"model":"gpt-5.1","stream":true,"prompt_cache_key":" key-1 ","service_tier":"fast"}`, string(patched))
}

func TestOpenAIUpstreamErrorBodyReadLimit(t *testing.T) {
	require.Equal(t, openAIUpstreamErrorBodyReadLimit, openAIUpstreamErrorBodyReadLimitForConfig(nil))

	cfg := &config.Config{}
	cfg.Gateway.LogUpstreamErrorBody = true
	cfg.Gateway.LogUpstreamErrorBodyMaxBytes = int(openAIUpstreamErrorBodyReadLimit) + 1024
	require.Equal(t, int64(cfg.Gateway.LogUpstreamErrorBodyMaxBytes), openAIUpstreamErrorBodyReadLimitForConfig(cfg))

	body := strings.Repeat("x", int(openAIUpstreamErrorBodyReadLimit)+1024)
	svc := &OpenAIGatewayService{}
	resp := &http.Response{Body: io.NopCloser(strings.NewReader(body))}
	got := svc.readUpstreamErrorBody(resp)
	require.Len(t, got, int(openAIUpstreamErrorBodyReadLimit))
}

func TestSanitizeEmptyBase64InputImagesInOpenAIRequestBodyMap(t *testing.T) {
	var reqBody map[string]any
	require.NoError(t, json.Unmarshal([]byte(`{
		"model":"gpt-5.4",
		"input":[
			{"role":"user","content":[
				{"type":"input_text","text":"Describe this"},
				{"type":"input_image","image_url":"data:image/png;base64,   "},
				{"type":"input_image","image_url":"data:image/png;base64,abc123"}
			]},
			{"role":"user","content":[
				{"type":"input_image","image_url":"data:image/png;base64,"}
			]},
			{"type":"input_image","image_url":"data:image/png;base64,"},
			{"type":"input_image","image_url":"data:image/png;base64,top-level-valid"}
		]
	}`), &reqBody))

	require.True(t, sanitizeEmptyBase64InputImagesInOpenAIRequestBodyMap(reqBody))

	normalized, err := json.Marshal(reqBody)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"model":"gpt-5.4",
		"input":[
			{"role":"user","content":[
				{"type":"input_text","text":"Describe this"},
				{"type":"input_image","image_url":"data:image/png;base64,abc123"}
			]},
			{"type":"input_image","image_url":"data:image/png;base64,top-level-valid"}
		]
	}`, string(normalized))
}

func TestSanitizeEmptyBase64InputImagesInOpenAIBody(t *testing.T) {
	body, changed, err := sanitizeEmptyBase64InputImagesInOpenAIBody([]byte(`{
		"model":"gpt-5.4",
		"stream":true,
		"input":[
			{"role":"user","content":[
				{"type":"input_text","text":"Describe this"},
				{"type":"input_image","image_url":"data:image/png;base64,"}
			]}
		]
	}`))
	require.NoError(t, err)
	require.True(t, changed)
	require.JSONEq(t, `{
		"model":"gpt-5.4",
		"stream":true,
		"input":[
			{"role":"user","content":[
				{"type":"input_text","text":"Describe this"}
			]}
		]
	}`, string(body))
}
