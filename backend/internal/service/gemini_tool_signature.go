package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const (
	geminiSignatureScopeKey = "gemini_tool_signature_scope"
	geminiSignatureSeedKey  = "gemini_tool_signature_seed"
	geminiSignatureTTL      = 14 * 24 * time.Hour
	geminiSignatureMaxCalls = 128
	geminiSignatureMaxBytes = 32 * 1024
	geminiArgumentsMaxBytes = 1 << 20
)

// Optional capability on GatewayCache; signatures survive gateway restarts.
type GeminiToolSignatureCache interface {
	GetGeminiToolSignatures(context.Context, string, []string) (map[string]string, error)
	PutGeminiToolSignatures(context.Context, string, map[string]string, time.Duration) error
}

func geminiSignatureDigest(parts ...string) string {
	b, _ := json.Marshal(parts)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func geminiToolCallKey(id, name, arguments string) string {
	if id == "" || name == "" || len(arguments) > geminiArgumentsMaxBytes || !json.Valid([]byte(arguments)) {
		return ""
	}
	// Preserve numeric precision; equivalent JSON object key order is not a new call.
	decoder := json.NewDecoder(strings.NewReader(arguments))
	decoder.UseNumber()
	var value any
	if decoder.Decode(&value) != nil {
		return ""
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return geminiSignatureDigest(id, name, string(canonical))
}

func (s *OpenAIGatewayService) restoreGeminiToolSignatures(ctx context.Context, c *gin.Context, account *Account, body []byte) []byte {
	if s == nil || c == nil || account == nil || account.Platform != PlatformGemini || account.ID <= 0 {
		return body
	}
	rawAPIKey, _ := c.Get("api_key")
	apiKey, ok := rawAPIKey.(*APIKey)
	if !ok || apiKey == nil || apiKey.ID <= 0 {
		return body
	}
	model := gjson.GetBytes(body, "model").String()
	keyID, groupID := fmt.Sprint(apiKey.ID), fmt.Sprint(derefGroupID(apiKey.GroupID))
	legacyScope := func() string {
		seed := explicitOpenAISessionID(c, body)
		if seed == "" {
			seed = deriveOpenAIContentSessionSeed(body)
		}
		return geminiSignatureDigest(keyID, groupID, fmt.Sprint(account.ID), account.GetCredential("api_key"),
			account.GetCredential("access_token"), model, seed)
	}
	shared := isGeminiOpenAIChatTarget(account, account.GetOpenAIBaseURL())
	var scope string
	if shared {
		seed := explicitOpenAISessionID(c, body)
		if seed == "" {
			seed = c.GetString(geminiSignatureSeedKey)
		}
		if seed == "" {
			// 工具列表和系统提示可能在会话中途变化，不能用它们作为签名缓存边界。
			firstUser := gjson.GetBytes(body, `messages.#(role=="user").content`)
			if !firstUser.Exists() {
				return body
			}
			seed = contentSessionSeedPrefix + normalizeCompatSeedJSON(json.RawMessage(firstUser.Raw))
		}
		scope = geminiSignatureDigest(keyID, groupID, model, seed)
	} else {
		scope = legacyScope()
	}
	c.Set(geminiSignatureScopeKey, scope)
	cache, ok := s.cache.(GeminiToolSignatureCache)
	if !ok {
		return body
	}
	paths := make(map[string][]string)
	messages := gjson.GetBytes(body, "messages").Array()
	// Prefer the active/recent tool turns when a long conversation exceeds the cap.
	for i := len(messages) - 1; i >= 0; i-- {
		message := messages[i]
		if message.Get("role").String() != "assistant" {
			continue
		}
		calls := message.Get("tool_calls").Array()
		for j := len(calls) - 1; j >= 0; j-- {
			call := calls[j]
			if call.Get("type").String() != "function" || call.Get("extra_content.google.thought_signature").String() != "" {
				continue
			}
			key := geminiToolCallKey(call.Get("id").String(), call.Get("function.name").String(), call.Get("function.arguments").String())
			if key != "" && len(paths) < geminiSignatureMaxCalls {
				paths[key] = append(paths[key], fmt.Sprintf("messages.%d.tool_calls.%d.extra_content.google.thought_signature", i, j))
			}
		}
	}
	if len(paths) == 0 {
		return body
	}
	keys := make([]string, 0, len(paths))
	for key := range paths {
		keys = append(keys, key)
	}
	cacheCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	found, err := cache.GetGeminiToolSignatures(cacheCtx, scope, keys)
	if err != nil {
		slog.Warn("gemini_tool_signature_cache_read_failed", "account_id", account.ID, "error", err)
		return body
	}
	if shared && len(found) == 0 {
		// 升级后首次续接旧会话时，优先从当前账号的旧缓存取回并写入共享缓存。
		if legacy, err := cache.GetGeminiToolSignatures(cacheCtx, legacyScope(), keys); err == nil {
			found = legacy
		}
	}
	updated := body
	restored := 0
	refresh := make(map[string]string)
	for key, signature := range found {
		if signature == "" || len(signature) > geminiSignatureMaxBytes {
			continue
		}
		for _, path := range paths[key] {
			next, err := sjson.SetBytes(updated, path, signature)
			if err != nil {
				return body
			}
			updated = next
			restored++
			refresh[key] = signature
		}
	}
	if len(refresh) > 0 {
		if err := cache.PutGeminiToolSignatures(cacheCtx, scope, refresh, geminiSignatureTTL); err != nil {
			slog.Warn("gemini_tool_signature_cache_refresh_failed", "account_id", account.ID, "error", err)
		}
	}
	slog.Info("gemini_tool_signature_lookup", "account_id", account.ID, "missing_calls", len(paths), "restored_count", restored)
	return updated
}

type geminiSignatureCall struct {
	id, name, arguments, signature string
	discarded                      bool
}

type geminiSignatureCollector struct {
	ctx   context.Context
	cache GeminiToolSignatureCache
	scope string
	calls map[string]*geminiSignatureCall
}

func (s *OpenAIGatewayService) newGeminiSignatureCollector(c *gin.Context, account *Account) *geminiSignatureCollector {
	if s == nil || c == nil || account == nil || account.Platform != PlatformGemini {
		return nil
	}
	cache, ok := s.cache.(GeminiToolSignatureCache)
	scope, _ := c.Get(geminiSignatureScopeKey)
	value, _ := scope.(string)
	if !ok || value == "" {
		return nil
	}
	ctx := context.Background()
	if c.Request != nil {
		ctx = context.WithoutCancel(c.Request.Context())
	}
	return &geminiSignatureCollector{ctx: ctx, cache: cache, scope: value, calls: make(map[string]*geminiSignatureCall)}
}

// Observe only genuine upstream responses, never client-supplied signatures.
func (collector *geminiSignatureCollector) observe(body []byte) {
	if collector == nil {
		return
	}
	for i, choice := range gjson.GetBytes(body, "choices").Array() {
		choiceIndex := choice.Get("index").Int()
		if !choice.Get("index").Exists() {
			choiceIndex = int64(i)
		}
		delta := choice.Get("delta")
		stream := delta.Exists()
		message := delta
		if !stream {
			message = choice.Get("message")
		}
		for j, tool := range message.Get("tool_calls").Array() {
			index := tool.Get("index").Int()
			if !tool.Get("index").Exists() {
				index = int64(j)
			}
			slot := fmt.Sprintf("%d:%d", choiceIndex, index)
			call := collector.calls[slot]
			if call == nil {
				if len(collector.calls) >= geminiSignatureMaxCalls {
					continue
				}
				call = &geminiSignatureCall{}
				collector.calls[slot] = call
			}
			if call.discarded || (tool.Get("type").Exists() && tool.Get("type").String() != "function") {
				continue
			}
			if id := tool.Get("id").String(); id != "" {
				if call.id != "" && call.id != id {
					call.discarded = true
					continue
				}
				call.id = id
			}
			call.name += tool.Get("function.name").String()
			call.arguments += tool.Get("function.arguments").String()
			if signature := tool.Get("extra_content.google.thought_signature").String(); signature != "" {
				call.signature = signature
			}
			if len(call.arguments) > geminiArgumentsMaxBytes || len(call.signature) > geminiSignatureMaxBytes || len(call.name) > 1024 || len(call.id) > 1024 {
				// Drop oversized calls rather than retaining unbounded streaming buffers.
				call.id, call.name, call.arguments, call.signature = "", "", "", ""
				call.discarded = true
			}
		}
		reason := choice.Get("finish_reason").String()
		if reason != "tool_calls" && reason != "stop" {
			continue
		}
		values := make(map[string]string)
		prefix := fmt.Sprintf("%d:", choiceIndex)
		for slot, call := range collector.calls {
			if !strings.HasPrefix(slot, prefix) {
				continue
			}
			if !call.discarded && call.signature != "" {
				if key := geminiToolCallKey(call.id, call.name, call.arguments); key != "" {
					values[key] = call.signature
				}
			}
			delete(collector.calls, slot)
		}
		if len(values) > 0 {
			ctx, cancel := context.WithTimeout(collector.ctx, time.Second)
			err := collector.cache.PutGeminiToolSignatures(ctx, collector.scope, values, geminiSignatureTTL)
			cancel()
			if err != nil {
				slog.Warn("gemini_tool_signature_cache_write_failed", "error", err)
			} else {
				slog.Info("gemini_tool_signatures_cached", "call_count", len(values))
			}
		}
	}
}
