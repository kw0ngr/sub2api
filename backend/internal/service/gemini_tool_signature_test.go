package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type geminiSignatureCacheStub struct {
	GatewayCache
	values map[string]map[string]string
	err    error
	ttl    time.Duration
}

func (cache *geminiSignatureCacheStub) GetGeminiToolSignatures(_ context.Context, scope string, keys []string) (map[string]string, error) {
	out := map[string]string{}
	for _, key := range keys {
		if signature, ok := cache.values[scope][key]; ok {
			out[key] = signature
		}
	}
	return out, cache.err
}

func (cache *geminiSignatureCacheStub) PutGeminiToolSignatures(_ context.Context, scope string, values map[string]string, ttl time.Duration) error {
	if cache.err != nil {
		return cache.err
	}
	if cache.values == nil {
		cache.values = map[string]map[string]string{}
	}
	if cache.values[scope] == nil {
		cache.values[scope] = map[string]string{}
	}
	for k, v := range values {
		cache.values[scope][k] = v
	}
	cache.ttl = ttl
	return nil
}

func signatureTestContext(keyID, groupID int64) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Request.Header.Set("session_id", "same-session")
	c.Set("api_key", &APIKey{ID: keyID, GroupID: &groupID})
	return c
}

func signatureTestAccount() *Account {
	return &Account{ID: 1833, Platform: PlatformGemini, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "upstream-key", "base_url": "https://generativelanguage.googleapis.com"}}
}

const signatureTestInput = `{"model":"gemini-3.8-flash","messages":[{"role":"user","content":"lookup value"}]}`
const signatureTestFollow = `{"model":"gemini-3.8-flash","messages":[{"role":"user","content":"lookup value"},{"role":"assistant","tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{\"large\":1234567890123456789,\"a\":1}"},"extra_content":{"other":{"keep":true}}}]},{"role":"tool","tool_call_id":"call_1","content":"OK"}]}`
const signatureTestResponse = `{"choices":[{"index":0,"message":{"tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{\"a\":1,\"large\":1234567890123456789}"},"extra_content":{"google":{"thought_signature":"genuine-signature"}}}]},"finish_reason":"tool_calls"}]}`

func TestGeminiToolSignaturesRestoreDevaMissingMetadata(t *testing.T) {
	cache := &geminiSignatureCacheStub{}
	svc := &OpenAIGatewayService{cache: cache, cfg: &config.Config{}}
	account := signatureTestAccount()
	c := signatureTestContext(33, 17)
	_, err := svc.buildUpstreamRequestOpenAICompatibleChatCompletions(context.Background(), c, account, []byte(signatureTestInput), "test")
	require.NoError(t, err)
	resp := &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(strings.TrimSuffix(signatureTestResponse, "}") + `,"usage":{"prompt_tokens":10,"completion_tokens":2}}`))}
	_, err = svc.handleNonStreamingResponse(context.Background(), resp, c, account, "gemini-3.8-flash", "gemini-3.8-flash")
	require.NoError(t, err)
	require.Equal(t, geminiSignatureTTL, cache.ttl)
	// New service instance: no process-local state is needed after a restart.
	svc = &OpenAIGatewayService{cache: cache, cfg: &config.Config{}}
	c = signatureTestContext(33, 17)
	input := []byte(signatureTestFollow)
	req, err := svc.buildUpstreamRequestOpenAICompatibleChatCompletions(context.Background(), c, account, input, "test")
	require.NoError(t, err)
	out, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	require.Equal(t, "genuine-signature", gjson.GetBytes(out, "messages.1.tool_calls.0.extra_content.google.thought_signature").String())
	require.Equal(t, gjson.GetBytes(input, "messages.1.tool_calls.0.function.arguments").String(), gjson.GetBytes(out, "messages.1.tool_calls.0.function.arguments").String())
	require.True(t, gjson.GetBytes(out, "messages.1.tool_calls.0.extra_content.other.keep").Bool())
	require.Equal(t, []byte(signatureTestFollow), input)
}

func TestGeminiToolSignatureStreamFragmentsAndTerminal(t *testing.T) {
	cache := &geminiSignatureCacheStub{}
	svc := &OpenAIGatewayService{cache: cache, cfg: &config.Config{}}
	c := signatureTestContext(33, 17)
	account := signatureTestAccount()
	svc.restoreGeminiToolSignatures(context.Background(), c, account, []byte(signatureTestInput))
	body := "data: " + `{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{\"a\":"}}]},"finish_reason":null}]}` + "\n\n" +
		"data: " + `{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"1,\"large\":1234567890123456789}"},"extra_content":{"google":{"thought_signature":"genuine-signature"}}}]},"finish_reason":null}]}` + "\n\n" +
		"data: " + `{"choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":2}}` + "\n\ndata: [DONE]\n\n"
	resp := &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}
	_, err := svc.handleStreamingResponse(context.Background(), resp, c, account, time.Now(), "gemini-3.8-flash", "gemini-3.8-flash")
	require.NoError(t, err)
	out := svc.restoreGeminiToolSignatures(context.Background(), signatureTestContext(33, 17), account, []byte(signatureTestFollow))
	require.Equal(t, "genuine-signature", gjson.GetBytes(out, "messages.1.tool_calls.0.extra_content.google.thought_signature").String())
}

func TestGeminiToolSignatureDevaWithoutSessionHeader(t *testing.T) {
	cache := &geminiSignatureCacheStub{}
	svc := &OpenAIGatewayService{cache: cache}
	account := signatureTestAccount()
	c := signatureTestContext(33, 17)
	c.Request.Header.Del("session_id")
	svc.restoreGeminiToolSignatures(context.Background(), c, account, []byte(signatureTestInput))
	svc.newGeminiSignatureCollector(c, account).observe([]byte(signatureTestResponse))
	next := signatureTestContext(33, 17)
	next.Request.Header.Del("session_id")
	out := svc.restoreGeminiToolSignatures(context.Background(), next, account, []byte(signatureTestFollow))
	require.Equal(t, "genuine-signature", gjson.GetBytes(out, "messages.1.tool_calls.0.extra_content.google.thought_signature").String())
	changedTools := strings.Replace(signatureTestFollow, `"messages":`, `"tools":[{"type":"function","name":"lookup"}],"messages":`, 1)
	require.Contains(t, string(svc.restoreGeminiToolSignatures(context.Background(), next, account, []byte(changedTools))), "genuine-signature")
	other := []byte(strings.Replace(signatureTestFollow, "lookup value", "different conversation", 1))
	require.Equal(t, other, svc.restoreGeminiToolSignatures(context.Background(), next, account, other))
}

func TestGeminiSignatureMigratesLegacyCacheOnCurrentAccount(t *testing.T) {
	cache := &geminiSignatureCacheStub{}
	svc := &OpenAIGatewayService{cache: cache}
	account := signatureTestAccount()
	legacy := geminiSignatureDigest("33", "17", fmt.Sprint(account.ID), account.GetCredential("api_key"), "", "gemini-3.8-flash", "same-session")
	key := geminiToolCallKey("call_1", "lookup", `{"a":1,"large":1234567890123456789}`)
	require.NoError(t, cache.PutGeminiToolSignatures(context.Background(), legacy, map[string]string{key: "genuine-signature"}, geminiSignatureTTL))
	input := []byte(signatureTestFollow)
	out := svc.restoreGeminiToolSignatures(context.Background(), signatureTestContext(33, 17), account, input)
	require.Contains(t, string(out), "genuine-signature")
	account.ID++
	account.Credentials["api_key"] = "other-key"
	require.Equal(t, out, svc.restoreGeminiToolSignatures(context.Background(), signatureTestContext(33, 17), account, input))
}

func TestGeminiToolSignatureIsolationAndCacheMiss(t *testing.T) {
	cache := &geminiSignatureCacheStub{}
	svc := &OpenAIGatewayService{cache: cache}
	account := signatureTestAccount()
	c := signatureTestContext(33, 17)
	svc.restoreGeminiToolSignatures(context.Background(), c, account, []byte(signatureTestInput))
	svc.newGeminiSignatureCollector(c, account).observe([]byte(signatureTestResponse))
	for _, kind := range []string{"tenant", "group", "model", "session", "id", "name", "arguments", "other_platform", "no_auth"} {
		t.Run(kind, func(t *testing.T) {
			input := []byte(signatureTestFollow)
			c := signatureTestContext(33, 17)
			a := signatureTestAccount()
			switch kind {
			case "tenant":
				c = signatureTestContext(34, 17)
			case "group":
				c = signatureTestContext(33, 18)
			case "model":
				input = []byte(strings.ReplaceAll(string(input), "gemini-3.8-flash", "gemini-4-argon"))
			case "session":
				c.Request.Header.Set("session_id", "other-session")
			case "id":
				input = []byte(strings.ReplaceAll(string(input), "call_1", "call_2"))
			case "name":
				input = []byte(strings.ReplaceAll(string(input), "lookup", "changed"))
			case "arguments":
				input = []byte(strings.ReplaceAll(string(input), "1234567890123456789", "1234567890123456788"))
			case "other_platform":
				a.Platform = PlatformOpenAI
			case "no_auth":
				c.Keys = nil
			}
			require.Equal(t, input, svc.restoreGeminiToolSignatures(context.Background(), c, a, input))
		})
	}
	cache.err = errors.New("redis unavailable")
	require.Equal(t, []byte(signatureTestFollow), svc.restoreGeminiToolSignatures(context.Background(), c, account, []byte(signatureTestFollow)))
}

func TestGeminiToolSignaturesNeverReplaceOrLearnClientSignatures(t *testing.T) {
	cache := &geminiSignatureCacheStub{}
	svc := &OpenAIGatewayService{cache: cache}
	account := signatureTestAccount()
	c := signatureTestContext(33, 17)
	input := []byte(strings.Replace(signatureTestFollow, `"other":{"keep":true}`, `"google":{"thought_signature":"client-original"},"other":{"keep":true}`, 1))
	require.Equal(t, input, svc.restoreGeminiToolSignatures(context.Background(), c, account, input))
	require.Empty(t, cache.values)
	svc.newGeminiSignatureCollector(c, account).observe([]byte(signatureTestResponse))
	require.Equal(t, input, svc.restoreGeminiToolSignatures(context.Background(), c, account, input))
}

func TestGeminiToolSignatureIncompleteOrOversizedNotCached(t *testing.T) {
	for _, kind := range []string{"length", "content_filter", "no_terminal", "oversized", "invalid_arguments"} {
		t.Run(kind, func(t *testing.T) {
			cache := &geminiSignatureCacheStub{}
			collector := &geminiSignatureCollector{ctx: context.Background(), cache: cache, scope: "test", calls: map[string]*geminiSignatureCall{}}
			body := signatureTestResponse
			switch kind {
			case "length", "content_filter":
				body = strings.Replace(body, `"finish_reason":"tool_calls"`, `"finish_reason":"`+kind+`"`, 1)
			case "no_terminal":
				body = strings.Replace(body, `"finish_reason":"tool_calls"`, `"finish_reason":null`, 1)
			case "oversized":
				body = strings.Replace(body, "genuine-signature", strings.Repeat("x", geminiSignatureMaxBytes+1), 1)
			case "invalid_arguments":
				body = strings.Replace(body, `{\"a\":1,\"large\":1234567890123456789}`, `{`, 1)
			}
			collector.observe([]byte(body))
			require.Empty(t, cache.values)
		})
	}
}

func TestGeminiSignatureRefreshAcrossAccounts(t *testing.T) {
	require.Equal(t, 14*24*time.Hour, geminiSignatureTTL)
	cache := &geminiSignatureCacheStub{}
	svc := &OpenAIGatewayService{cache: cache}
	account := signatureTestAccount()
	c := signatureTestContext(33, 17)
	svc.restoreGeminiToolSignatures(context.Background(), c, account, []byte(signatureTestInput))
	svc.newGeminiSignatureCollector(c, account).observe([]byte(signatureTestResponse))
	cache.ttl = 0
	other := signatureTestAccount()
	other.ID++
	other.Credentials["api_key"] = "another-project-key"
	out := svc.restoreGeminiToolSignatures(context.Background(), signatureTestContext(33, 17), other, []byte(signatureTestFollow))
	require.Contains(t, string(out), "genuine-signature")
	require.Equal(t, 14*24*time.Hour, cache.ttl, "活跃会话续期缓存中的真实签名")
}

func TestGeminiSignatureFallbackOnlyChecksActiveTurn(t *testing.T) {
	const signaturePath = "messages.1.tool_calls.0.extra_content.google.thought_signature"
	out := normalizeGeminiOpenAIChatRequest([]byte(signatureTestFollow))
	require.Equal(t, "skip_thought_signature_validator", gjson.GetBytes(out, signaturePath).String())
	require.True(t, gjson.GetBytes(out, "messages.1.tool_calls.0.extra_content.other.keep").Bool())
	// 新的用户回合开始之后，不给老调用或并行调用的后续项补占位签名。
	old := strings.TrimSuffix(signatureTestFollow, "]}") + `,{"role":"user","content":"New task"}]}`
	require.Equal(t, []byte(old), normalizeGeminiOpenAIChatRequest([]byte(old)))
	signed := strings.Replace(signatureTestFollow, `"other":{"keep":true}`, `"google":{"thought_signature":"valid"}`, 1)
	require.Equal(t, []byte(signed), normalizeGeminiOpenAIChatRequest([]byte(signed)))
	legacy := strings.ReplaceAll(signatureTestFollow, "gemini-3.8-flash", "gemini-2.5-flash")
	require.Equal(t, []byte(legacy), normalizeGeminiOpenAIChatRequest([]byte(legacy)))
	parallel := `{"model":"gemini-3.8-flash","messages":[{"role":"user","content":"hi"},{"role":"assistant","tool_calls":[{"type":"function","extra_content":{"google":{"thought_signature":"first-call-signature"}}},{"type":"function"}]},{"role":"tool","content":"OK"}]}`
	require.Equal(t, []byte(parallel), normalizeGeminiOpenAIChatRequest([]byte(parallel)))
}

func TestGeminiSignatureLookupPrioritizesNewestCallsInLongHistory(t *testing.T) {
	cache := &geminiSignatureCacheStub{}
	svc := &OpenAIGatewayService{cache: cache}
	account := signatureTestAccount()
	c := signatureTestContext(33, 17)
	svc.restoreGeminiToolSignatures(context.Background(), c, account, []byte(signatureTestInput))
	svc.newGeminiSignatureCollector(c, account).observe([]byte(signatureTestResponse))
	messages := []any{map[string]any{"role": "user", "content": "lookup value"}}
	for i := 0; i < geminiSignatureMaxCalls+5; i++ {
		messages = append(messages, map[string]any{"role": "assistant", "tool_calls": []any{
			map[string]any{"id": fmt.Sprintf("old-%d", i), "type": "function", "function": map[string]any{"name": "lookup", "arguments": "{}"}},
		}})
	}
	var follow map[string]any
	require.NoError(t, json.Unmarshal([]byte(signatureTestFollow), &follow))
	newest := follow["messages"].([]any)[1]
	messages = append(messages, newest)
	body, err := json.Marshal(map[string]any{"model": "gemini-3.8-flash", "messages": messages})
	require.NoError(t, err)
	out := svc.restoreGeminiToolSignatures(context.Background(), c, account, body)
	require.Equal(t, "genuine-signature", gjson.GetBytes(out, fmt.Sprintf("messages.%d.tool_calls.0.extra_content.google.thought_signature", len(messages)-1)).String())
}
