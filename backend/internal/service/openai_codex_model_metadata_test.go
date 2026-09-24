package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCodexManifestEveryRequiredKeyForAstraAndSol(t *testing.T) {
	// Given
	groupID := int64(11)
	svc := &OpenAIGatewayService{accountRepo: localCodexModelsAccountRepoStub{accounts: []Account{{
		ID: 621, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true,
		Credentials: map[string]any{"model_mapping": map[string]any{
			"gpt-6-astra": "gpt-6-astra",
			"gpt-5.6-sol": "gpt-5.6-sol",
		}},
	}}}}

	// When
	manifest, err := svc.BuildLocalCodexModelsManifest(context.Background(), &groupID, "")

	// Then
	require.NoError(t, err)
	models := task5DecodeCodexModels(t, manifest.Body)
	require.Len(t, models, 2)
	bySlug := task5ModelsBySlug(models)
	task5RequireCompleteOpenAIModelInfo(t, bySlug["gpt-6-astra"], task5ManifestWant{
		slug: "gpt-6-astra", display: "GPT-6 Astra", defaultReasoning: "medium", supportsSearch: true,
		applyPatch: "freeform", compHash: "3000", toolMode: nil,
	})
	task5RequireCompleteOpenAIModelInfo(t, bySlug["gpt-5.6-sol"], task5ManifestWant{
		slug: "gpt-5.6-sol", display: "GPT-5.6 Sol", defaultReasoning: "low", supportsSearch: false,
		applyPatch: nil, compHash: nil, toolMode: nil,
	})
}

func TestGPT6SolLunaUpstreamMetadataKeepsNone(t *testing.T) {
	for _, modelID := range []string{"gpt-6-sol", "gpt-6-luna"} {
		model := buildLocalCodexModelFromSpec(localCodexModelSpec{
			Slug: modelID,
			UpstreamMetadata: []UpstreamModelMetadata{{
				ID: modelID, Reasoning: task6BoolPtr(true),
				DefaultReasoningLevel: "none", SupportedReasoningLevels: []string{"none", "low", "max"},
			}},
		}, 1)
		require.Equal(t, "none", model.DefaultReasoningLevel)
		require.Equal(t, []string{"none", "low", "max"}, localCodexModelEfforts(model))
	}
}

func TestCodexManifestBareGPT56EmitsCanonicalSolSlugNoDuplicate(t *testing.T) {
	// Given
	groupID := int64(12)
	svc := &OpenAIGatewayService{accountRepo: localCodexModelsAccountRepoStub{accounts: []Account{{
		ID: 622, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true,
		Credentials: map[string]any{"model_mapping": map[string]any{
			"gpt-5.6":     "gpt-5.6-sol",
			"gpt5.6":      "gpt-5.6-sol",
			"gpt-5.6-sol": "gpt-5.6-sol",
		}},
	}}}}

	// When
	manifest, err := svc.BuildLocalCodexModelsManifest(context.Background(), &groupID, "")

	// Then
	require.NoError(t, err)
	models := task5DecodeCodexModels(t, manifest.Body)
	require.Len(t, models, 1)
	require.Equal(t, "gpt-5.6-sol", models[0]["slug"])
	require.NotContains(t, string(manifest.Body), `"slug":"gpt-5.6"`)
	require.NotContains(t, string(manifest.Body), `"slug":"gpt5.6"`)
}

func TestBuildLocalCodexModelsManifestPreservesExplicitPublicAliasOnce(t *testing.T) {
	// Given
	groupID := int64(13)
	svc := &OpenAIGatewayService{accountRepo: localCodexModelsAccountRepoStub{accounts: []Account{{
		ID: 623, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true,
		Credentials: map[string]any{"model_mapping": map[string]any{"team-sol": "gpt-5.6-sol"}},
	}}}}

	// When
	manifest, err := svc.BuildLocalCodexModelsManifest(context.Background(), &groupID, "")

	// Then
	require.NoError(t, err)
	models := task5DecodeCodexModels(t, manifest.Body)
	require.Len(t, models, 1)
	require.Equal(t, "team-sol", models[0]["slug"])
	require.Equal(t, "low", models[0]["default_reasoning_level"])
	require.Equal(t, []string{"low", "medium", "high", "xhigh", "max"}, task5ReasoningEfforts(t, models[0]))
}

func TestFetchCodexModelsManifestConvertsOrdinaryOpenAIListToCompleteManifest(t *testing.T) {
	// Given
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("ETag", `"ordinary-list"`)
		_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"gpt-6-astra"},{"id":"company-coding-model"}]}`))
	}))
	defer srv.Close()
	oldURL := chatgptCodexModelsURL
	chatgptCodexModelsURL = srv.URL
	t.Cleanup(func() { chatgptCodexModelsURL = oldURL })
	svc := &OpenAIGatewayService{}
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"access_token": "token"}}

	// When
	manifest, err := svc.FetchCodexModelsManifest(context.Background(), account, "0.153.4", "")

	// Then
	require.NoError(t, err)
	require.NotContains(t, string(manifest.Body), `"object":"list"`)
	models := task5DecodeCodexModels(t, manifest.Body)
	bySlug := task5ModelsBySlug(models)
	require.Contains(t, bySlug, "gpt-6-astra")
	require.Contains(t, bySlug, "company-coding-model")
	task5RequireKeys(t, bySlug["company-coding-model"], task5RequiredModelInfoKeys())
}

func TestFetchCodexModelsManifestFiltersWireNoneReasoningButKeepsMax(t *testing.T) {
	// Given
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"gpt-5.6-sol","reasoning":true,"default_reasoning_level":"none","supported_reasoning_levels":["none","low","medium","high","xhigh","max"]}]}`))
	}))
	defer srv.Close()
	oldURL := chatgptCodexModelsURL
	chatgptCodexModelsURL = srv.URL
	t.Cleanup(func() { chatgptCodexModelsURL = oldURL })
	svc := &OpenAIGatewayService{}
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"access_token": "token"}}

	// When
	manifest, err := svc.FetchCodexModelsManifest(context.Background(), account, "0.153.4", "")

	// Then
	require.NoError(t, err)
	models := task5DecodeCodexModels(t, manifest.Body)
	require.Len(t, models, 1)
	require.Equal(t, []string{"low", "medium", "high", "xhigh", "max"}, task5ReasoningEfforts(t, models[0]))
	require.NotContains(t, task5ReasoningEfforts(t, models[0]), "none")
}

func TestFetchCodexModelsManifestPreservesOAuthResponsesLiteOverlay(t *testing.T) {
	// Given
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"gpt-6-astra","use_responses_lite":true,"apply_patch_tool_type":"freeform","tool_mode":"code_mode_only"}]}`))
	}))
	defer srv.Close()
	oldURL := chatgptCodexModelsURL
	chatgptCodexModelsURL = srv.URL
	t.Cleanup(func() { chatgptCodexModelsURL = oldURL })
	svc := &OpenAIGatewayService{}
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"access_token": "token"}}

	// When
	manifest, err := svc.FetchCodexModelsManifest(context.Background(), account, "0.153.4", "")

	// Then
	require.NoError(t, err)
	models := task5DecodeCodexModels(t, manifest.Body)
	require.Len(t, models, 1)
	useResponsesLite, ok := models[0]["use_responses_lite"].(bool)
	require.True(t, ok)
	require.True(t, useResponsesLite)
	require.Equal(t, "code_mode_only", models[0]["tool_mode"])
}

func TestFetchCodexModelsManifestIgnoresInvalidOverlayWithoutDroppingModel(t *testing.T) {
	// Given
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"gpt-6-astra","supported_reasoning_levels":[],"context_window":0,"input_modalities":[],"use_responses_lite":"yes"}]}`))
	}))
	defer srv.Close()
	oldURL := chatgptCodexModelsURL
	chatgptCodexModelsURL = srv.URL
	t.Cleanup(func() { chatgptCodexModelsURL = oldURL })
	svc := &OpenAIGatewayService{}
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"access_token": "token"}}

	// When
	manifest, err := svc.FetchCodexModelsManifest(context.Background(), account, "0.153.4", "")

	// Then
	require.NoError(t, err)
	models := task5DecodeCodexModels(t, manifest.Body)
	require.Len(t, models, 1)
	require.Equal(t, "gpt-6-astra", models[0]["slug"])
	require.Equal(t, []string{"low", "medium", "high", "xhigh", "max"}, task5ReasoningEfforts(t, models[0]))
	require.Equal(t, float64(1050000), models[0]["context_window"])
	useResponsesLite, ok := models[0]["use_responses_lite"].(bool)
	require.True(t, ok)
	require.False(t, useResponsesLite)
}

type task5ManifestWant struct {
	slug             string
	display          string
	defaultReasoning string
	supportsSearch   bool
	applyPatch       any
	compHash         any
	toolMode         any
}

func task5RequireCompleteOpenAIModelInfo(t *testing.T, model map[string]any, want task5ManifestWant) {
	t.Helper()
	task5RequireKeys(t, model, task5RequiredModelInfoKeys())
	require.NotContains(t, model, "max_output_tokens")
	require.Equal(t, want.slug, model["slug"])
	require.Equal(t, want.display, model["display_name"])
	require.Equal(t, "OpenAI GPT coding model routed through Sub2API.", model["description"])
	require.Equal(t, want.defaultReasoning, model["default_reasoning_level"])
	require.Equal(t, []string{"low", "medium", "high", "xhigh", "max"}, task5ReasoningEfforts(t, model))
	require.Equal(t, "unified_exec", model["shell_type"])
	require.Equal(t, "list", model["visibility"])
	require.Equal(t, true, model["supported_in_api"])
	require.Equal(t, []any{}, model["additional_speed_tiers"])
	require.Equal(t, []any{map[string]any{"id": "priority", "name": "Fast", "description": "Priority processing for lower latency."}}, model["service_tiers"])
	require.Nil(t, model["default_service_tier"])
	require.Nil(t, model["availability_nux"])
	require.Nil(t, model["upgrade"])
	modelMessages, ok := model["model_messages"].(map[string]any)
	require.True(t, ok)
	task5RequireKeys(t, modelMessages, []string{"instructions_template", "instructions_variables", "approvals", "collaboration_modes", "auto_review", "permissions", "multi_agent", "token_budget", "guardian_v2"})
	require.NotEmpty(t, modelMessages["instructions_template"])
	includeSkillsUsageInstructions, ok := model["include_skills_usage_instructions"].(bool)
	require.True(t, ok)
	require.False(t, includeSkillsUsageInstructions)
	includePluginUsageInstructions, ok := model["include_plugin_usage_instructions"].(bool)
	require.True(t, ok)
	require.False(t, includePluginUsageInstructions)
	includeAppsUsageInstructions, ok := model["include_apps_usage_instructions"].(bool)
	require.True(t, ok)
	require.False(t, includeAppsUsageInstructions)
	supportsReasoningSummaryParameter, ok := model["supports_reasoning_summary_parameter"].(bool)
	require.True(t, ok)
	require.True(t, supportsReasoningSummaryParameter)
	require.Equal(t, "none", model["default_reasoning_summary"])
	supportVerbosity, ok := model["support_verbosity"].(bool)
	require.True(t, ok)
	require.True(t, supportVerbosity)
	require.Equal(t, "low", model["default_verbosity"])
	require.Equal(t, want.applyPatch, model["apply_patch_tool_type"])
	require.Equal(t, "text", model["web_search_tool_type"])
	require.Equal(t, map[string]any{"mode": "tokens", "limit": float64(10000)}, model["truncation_policy"])
	supportsImageDetailOriginal, ok := model["supports_image_detail_original"].(bool)
	require.True(t, ok)
	require.False(t, supportsImageDetailOriginal)
	supportsParallelToolCalls, ok := model["supports_parallel_tool_calls"].(bool)
	require.True(t, ok)
	require.True(t, supportsParallelToolCalls)
	require.Equal(t, float64(1050000), model["context_window"])
	require.Equal(t, float64(1050000), model["max_context_window"])
	require.Nil(t, model["auto_compact_token_limit"])
	require.Equal(t, want.compHash, model["comp_hash"])
	require.Equal(t, float64(95), model["effective_context_window_percent"])
	require.Equal(t, []any{}, model["experimental_supported_tools"])
	require.Equal(t, []any{"text", "image"}, model["input_modalities"])
	require.Equal(t, want.supportsSearch, model["supports_search_tool"])
	useResponsesLite, ok := model["use_responses_lite"].(bool)
	require.True(t, ok)
	require.False(t, useResponsesLite)
	nodeREPLAutoReviewRequired, ok := model["node_repl_auto_review_required"].(bool)
	require.True(t, ok)
	require.False(t, nodeREPLAutoReviewRequired)
	nodeREPLDisabled, ok := model["node_repl_disabled"].(bool)
	require.True(t, ok)
	require.False(t, nodeREPLDisabled)
	require.Nil(t, model["auto_review_model_override"])
	require.Nil(t, model["model_specialty"])
	require.Equal(t, want.toolMode, model["tool_mode"])
	require.Nil(t, model["multi_agent_version"])
}

func task5DecodeCodexModels(t *testing.T, body []byte) []map[string]any {
	t.Helper()
	var envelope struct {
		Models []map[string]any `json:"models"`
	}
	require.NoError(t, json.Unmarshal(body, &envelope))
	return envelope.Models
}

func task5ModelsBySlug(models []map[string]any) map[string]map[string]any {
	bySlug := make(map[string]map[string]any, len(models))
	for _, model := range models {
		slug, _ := model["slug"].(string)
		bySlug[slug] = model
	}
	return bySlug
}

func task5ReasoningEfforts(t *testing.T, model map[string]any) []string {
	t.Helper()
	levels, ok := model["supported_reasoning_levels"].([]any)
	require.True(t, ok)
	efforts := make([]string, 0, len(levels))
	for _, raw := range levels {
		level, ok := raw.(map[string]any)
		require.True(t, ok)
		effort, ok := level["effort"].(string)
		require.True(t, ok)
		efforts = append(efforts, effort)
	}
	return efforts
}

func task5RequireKeys(t *testing.T, model map[string]any, want []string) {
	t.Helper()
	keys := make([]string, 0, len(model))
	for key := range model {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	sortedWant := append([]string(nil), want...)
	sort.Strings(sortedWant)
	require.Equal(t, sortedWant, keys)
}

func task5RequiredModelInfoKeys() []string {
	return []string{
		"additional_speed_tiers", "apply_patch_tool_type", "auto_compact_token_limit", "auto_review_model_override",
		"availability_nux", "comp_hash", "context_window", "default_reasoning_level", "default_reasoning_summary",
		"default_service_tier", "default_verbosity", "description", "display_name", "effective_context_window_percent",
		"experimental_supported_tools", "include_apps_usage_instructions", "include_plugin_usage_instructions",
		"include_skills_usage_instructions", "input_modalities", "max_context_window", "model_messages", "model_specialty",
		"multi_agent_version", "node_repl_auto_review_required", "node_repl_disabled", "priority", "service_tiers",
		"shell_type", "slug", "support_verbosity", "supported_in_api", "supported_reasoning_levels", "supports_image_detail_original",
		"supports_parallel_tool_calls", "supports_reasoning_summary_parameter", "supports_search_tool", "tool_mode", "truncation_policy",
		"upgrade", "use_responses_lite", "visibility", "web_search_tool_type",
	}
}

func TestModelMetadataProjectionUsesCanonicalAliasSnapshotAndHidesStale(t *testing.T) {
	// Given
	groupID := int64(706)
	svc := &OpenAIGatewayService{accountRepo: localCodexModelsAccountRepoStub{accounts: []Account{{
		ID: 706, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true,
		Credentials: map[string]any{"model_mapping": map[string]any{"my-astra": "gpt-6-astra"}},
		Extra: map[string]any{"upstream_model_metadata": map[string]any{"source": "upstream", "synced_at": "2026-09-06T00:00:00Z", "models": map[string]any{
			"gpt-6-astra": map[string]any{"id": "gpt-6-astra", "sources": []any{"upstream"}, "display_name": "Snapshot Astra", "description": "Snapshot description", "reasoning": true, "default_reasoning_level": "max", "supported_reasoning_levels": []any{"low", "max"}, "input_modalities": []any{"text"}, "context_window": float64(321000), "max_output_tokens": float64(128000), "codex_tool_capabilities": map[string]any{"supports_search_tool": false, "apply_patch_tool_type": nil, "comp_hash": "snapshot", "use_responses_lite": true}},
			"stale-model": map[string]any{"id": "stale-model", "sources": []any{"upstream"}, "reasoning": false, "input_modalities": []any{"text"}, "context_window": float64(64000)},
		}}},
	}}}}

	// When
	manifest, err := svc.BuildLocalCodexModelsManifest(context.Background(), &groupID, "")

	// Then
	require.NoError(t, err)
	models := task5DecodeCodexModels(t, manifest.Body)
	require.Len(t, models, 1)
	model := models[0]
	require.Equal(t, "my-astra", model["slug"])
	require.Equal(t, "Snapshot Astra", model["display_name"])
	require.Equal(t, "Snapshot description", model["description"])
	require.Equal(t, []string{"low", "max"}, task5ReasoningEfforts(t, model))
	require.Equal(t, "max", model["default_reasoning_level"])
	require.Equal(t, float64(321000), model["context_window"])
	supportsSearchTool, ok := model["supports_search_tool"].(bool)
	require.True(t, ok)
	require.False(t, supportsSearchTool)
	require.Nil(t, model["apply_patch_tool_type"])
	require.Equal(t, "snapshot", model["comp_hash"])
	useResponsesLite, ok := model["use_responses_lite"].(bool)
	require.True(t, ok)
	require.False(t, useResponsesLite, "API-key projection must keep Responses Lite disabled")
	require.NotContains(t, string(manifest.Body), "stale-model")
}

func TestMixedPoolCodexModelsUnionSnapshotCapabilitiesWithoutLoss(t *testing.T) {
	// Given
	groupID := int64(707)
	accounts := []Account{
		{ID: 7071, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Credentials: map[string]any{"model_mapping": map[string]any{"shared-code": "x-code"}}, Extra: map[string]any{"upstream_model_metadata": map[string]any{"source": "upstream", "synced_at": "2026-09-06T00:00:00Z", "models": map[string]any{"x-code": map[string]any{"id": "x-code", "sources": []any{"upstream"}, "display_name": "X Code", "reasoning": true, "default_reasoning_level": "high", "supported_reasoning_levels": []any{"high"}, "input_modalities": []any{"text"}, "context_window": float64(64000), "max_output_tokens": float64(4096), "codex_tool_capabilities": map[string]any{"supports_search_tool": true}}}}}},
		{ID: 7072, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Credentials: map[string]any{"model_mapping": map[string]any{"shared-code": "x-code"}}, Extra: map[string]any{"upstream_model_metadata": map[string]any{"source": "upstream", "synced_at": "2026-09-06T00:00:00Z", "models": map[string]any{"x-code": map[string]any{"id": "x-code", "sources": []any{"upstream"}, "reasoning": true, "supported_reasoning_levels": []any{"max"}, "input_modalities": []any{"text", "image"}, "context_window": float64(64000), "codex_tool_capabilities": map[string]any{"apply_patch_tool_type": "freeform", "comp_hash": "abc"}}}}}},
	}
	svc := &OpenAIGatewayService{accountRepo: localCodexModelsAccountRepoStub{accounts: accounts}}

	// When
	manifest, err := svc.BuildLocalCodexModelsManifest(context.Background(), &groupID, "")

	// Then
	require.NoError(t, err)
	models := task5DecodeCodexModels(t, manifest.Body)
	require.Len(t, models, 1)
	model := models[0]
	require.Equal(t, "shared-code", model["slug"])
	require.Equal(t, "X Code", model["display_name"])
	require.Equal(t, []string{"high", "max"}, task5ReasoningEfforts(t, model))
	require.Equal(t, []any{"text", "image"}, model["input_modalities"])
	supportsSearchTool, ok := model["supports_search_tool"].(bool)
	require.True(t, ok)
	require.True(t, supportsSearchTool)
	require.Equal(t, "freeform", model["apply_patch_tool_type"])
	require.Equal(t, "abc", model["comp_hash"])
}
