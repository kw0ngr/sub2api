package service

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	openaiapi "github.com/Wei-Shaw/sub2api/internal/pkg/openai"
)

func localCodexModelSpecsForAccounts(accounts []Account) ([]localCodexModelSpec, bool) {
	specBySlug := map[string]localCodexModelSpec{}
	hasAPIKeyAccount := false
	hasExplicitMapping := false
	for i := range accounts {
		account := &accounts[i]
		if !account.IsOpenAIApiKey() {
			continue
		}
		hasAPIKeyAccount = true
		for publicID, mappedID := range account.GetModelMapping() {
			spec, ok := localCodexModelSpecForMapping(publicID, mappedID)
			if !ok {
				continue
			}
			spec.ForceAPIKey = true
			if metadata, ok := account.GetUpstreamModelMetadata(spec.MetadataID); ok {
				spec.UpstreamMetadata = append(spec.UpstreamMetadata, metadata)
			}
			if existing, ok := specBySlug[spec.Slug]; ok {
				spec.UpstreamMetadata = append(existing.UpstreamMetadata, spec.UpstreamMetadata...)
			}
			specBySlug[spec.Slug] = spec
			hasExplicitMapping = true
		}
	}
	if !hasAPIKeyAccount {
		return nil, false
	}
	if !hasExplicitMapping {
		for _, modelID := range openaiapi.DefaultModelIDs() {
			spec, _ := localCodexModelSpecForMapping(modelID, modelID)
			spec.ForceAPIKey = true
			specBySlug[spec.Slug] = spec
		}
	}
	specs := make([]localCodexModelSpec, 0, len(specBySlug))
	for _, spec := range specBySlug {
		specs = append(specs, spec)
	}
	sortLocalCodexModelSpecs(specs)
	return specs, true
}

func localCodexModelSpecForMapping(publicID, mappedID string) (localCodexModelSpec, bool) {
	publicID = strings.TrimSpace(publicID)
	mappedID = strings.TrimSpace(mappedID)
	if publicID == "" || strings.Contains(publicID, "*") {
		return localCodexModelSpec{}, false
	}
	if mappedID == "" {
		mappedID = publicID
	}
	publicCanonical := normalizeKnownOpenAICodexModel(publicID)
	targetCanonical := normalizeKnownOpenAICodexModel(mappedID)
	metadataID := mappedID
	if targetCanonical != "" {
		metadataID = targetCanonical
	} else if publicCanonical != "" {
		metadataID = publicCanonical
	}
	slug := publicID
	if publicCanonical != "" && strings.EqualFold(publicCanonical, metadataID) {
		slug = publicCanonical
	}
	return localCodexModelSpec{Slug: slug, MetadataID: metadataID}, true
}

func sortLocalCodexModelSpecs(specs []localCodexModelSpec) {
	preferred := make(map[string]int, len(openaiapi.DefaultModels))
	for i, model := range openaiapi.DefaultModels {
		preferred[model.ID] = i
	}
	sort.Slice(specs, func(i, j int) bool {
		leftRank, leftPreferred := preferred[specs[i].Slug]
		rightRank, rightPreferred := preferred[specs[j].Slug]
		if leftPreferred != rightPreferred {
			return leftPreferred
		}
		if leftPreferred && leftRank != rightRank {
			return leftRank < rightRank
		}
		return specs[i].Slug < specs[j].Slug
	})
}

func buildCodexModelsManifestBody(specs []localCodexModelSpec, overlays map[string]map[string]json.RawMessage) ([]byte, error) {
	models := make([]localCodexModel, 0, len(specs))
	for i, spec := range specs {
		model := buildLocalCodexModelFromSpec(spec, i+1)
		if overlay := overlays[spec.Slug]; len(overlay) > 0 {
			applyOpenAIModelListOverlay(&model, overlay)
		}
		if spec.ForceAPIKey {
			model.UseResponsesLite = false
		}
		models = append(models, model)
	}
	return json.Marshal(struct {
		Models []localCodexModel `json:"models"`
	}{Models: models})
}

func localCodexModelsBodyETag(body []byte) string {
	digest := sha256.Sum256(body)
	return fmt.Sprintf(`W/"sub2api-%x"`, digest[:16])
}
