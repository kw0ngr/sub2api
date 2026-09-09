package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"
)

const UpstreamModelMetadataExtraKey = "upstream_model_metadata"

type UpstreamModelMetadata struct {
	ID                       string                     `json:"id"`
	Sources                  []string                   `json:"sources"`
	DisplayName              string                     `json:"display_name,omitempty"`
	Description              string                     `json:"description,omitempty"`
	Reasoning                *bool                      `json:"reasoning,omitempty"`
	DefaultReasoningLevel    string                     `json:"default_reasoning_level,omitempty"`
	SupportedReasoningLevels []string                   `json:"supported_reasoning_levels,omitempty"`
	InputModalities          []string                   `json:"input_modalities"`
	ContextWindow            int                        `json:"context_window"`
	MaxOutputTokens          int                        `json:"max_output_tokens,omitempty"`
	CodexToolCapabilities    map[string]json.RawMessage `json:"codex_tool_capabilities,omitempty"`
}

type UpstreamModelMetadataSnapshot struct {
	Source   string                           `json:"source"`
	SyncedAt string                           `json:"synced_at"`
	Models   map[string]UpstreamModelMetadata `json:"models"`
}

func (a *Account) SetUpstreamModelMetadataSnapshot(snapshot UpstreamModelMetadataSnapshot) {
	if a == nil {
		return
	}
	if a.Extra == nil {
		a.Extra = map[string]any{}
	}
	a.Extra[UpstreamModelMetadataExtraKey] = snapshot
}

func (a *Account) GetUpstreamModelMetadataSnapshot() *UpstreamModelMetadataSnapshot {
	if a == nil || a.Extra == nil || a.Extra[UpstreamModelMetadataExtraKey] == nil {
		return nil
	}
	body, err := json.Marshal(a.Extra[UpstreamModelMetadataExtraKey])
	if err != nil {
		return nil
	}
	var snapshot UpstreamModelMetadataSnapshot
	if err := json.Unmarshal(body, &snapshot); err != nil || len(snapshot.Models) == 0 {
		return nil
	}
	return &snapshot
}

func (a *Account) GetUpstreamModelMetadata(modelID string) (UpstreamModelMetadata, bool) {
	snapshot := a.GetUpstreamModelMetadataSnapshot()
	if snapshot == nil {
		return UpstreamModelMetadata{}, false
	}
	metadata, ok := snapshot.Models[canonicalUpstreamModelID(modelID)]
	return metadata, ok
}

func (s *AccountTestService) persistUpstreamModelMetadata(ctx context.Context, account *Account, listedModels []string, body []byte) error {
	if s == nil || s.accountRepo == nil || account == nil || account.ID <= 0 {
		return nil
	}
	liveMetadata := extractUpstreamModelMetadata(body)
	complete := map[string]UpstreamModelMetadata{}
	for _, modelID := range dedupeAndSortModelIDs(append(listedModels, configuredUpstreamModelTargets(account)...)) {
		canonical := canonicalUpstreamModelID(modelID)
		if canonical == "" || isCodexDedicatedMediaModel(canonical) {
			continue
		}
		metadata := liveMetadata[canonical]
		if strings.TrimSpace(metadata.ID) == "" {
			metadata = UpstreamModelMetadata{ID: canonical, Sources: []string{"upstream"}}
		}
		if fallback, ok := localOfficialUpstreamModelMetadata(canonical); ok {
			metadata = fillUpstreamModelMetadata(metadata, fallback)
		}
		if previous := account.GetUpstreamModelMetadataSnapshot(); previous != nil {
			if old, ok := previous.Models[canonical]; ok {
				metadata = fillUpstreamModelMetadata(metadata, old)
			}
		}
		metadata.ID = canonical
		metadata.Sources = normalizeMetadataSources(metadata.Sources)
		metadata.SupportedReasoningLevels = normalizeSnapshotReasoningLevelsForModel(canonical, metadata.SupportedReasoningLevels)
		metadata.InputModalities = normalizeCodexInputModalities(metadata.InputModalities)
		if upstreamModelMetadataIsComplete(canonical, metadata) {
			complete[canonical] = metadata
		}
	}
	if len(complete) == 0 {
		return nil
	}
	if previous := account.GetUpstreamModelMetadataSnapshot(); previous != nil {
		for modelID, old := range previous.Models {
			canonical := canonicalUpstreamModelID(modelID)
			if canonical == "" || isCodexDedicatedMediaModel(canonical) || !upstreamModelMetadataIsComplete(canonical, old) {
				continue
			}
			if _, ok := complete[canonical]; !ok {
				old.ID = canonical
				complete[canonical] = old
			}
		}
	}
	snapshot := UpstreamModelMetadataSnapshot{Source: metadataSnapshotSource(complete), SyncedAt: time.Now().UTC().Format(time.RFC3339), Models: complete}
	if err := s.accountRepo.UpdateExtra(ctx, account.ID, map[string]any{UpstreamModelMetadataExtraKey: snapshot}); err != nil {
		return newUpstreamModelSyncInternalError("Failed to save upstream model metadata", err)
	}
	account.SetUpstreamModelMetadataSnapshot(snapshot)
	return nil
}
