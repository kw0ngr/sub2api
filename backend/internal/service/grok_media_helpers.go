package service

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/tidwall/gjson"
)

func (u OpenAIImagesUpload) ModerationDataURL() string {
	dataURL, err := openAIImageUploadToDataURL(u)
	if err != nil {
		return ""
	}
	return dataURL
}

func NormalizeImageBillingTierOrDefault(size string) string {
	return normalizeOpenAIImageSizeTier(size)
}

func NormalizeVideoBillingResolutionOrDefault(resolution string) string {
	switch strings.ToLower(strings.TrimSpace(resolution)) {
	case "720p", "1080p":
		return strings.ToLower(strings.TrimSpace(resolution))
	default:
		return "720p"
	}
}

func NormalizeVideoBillingDurationSecondsOrDefault(seconds int) int {
	if seconds <= 0 {
		return 6
	}
	if seconds > 15 {
		return 15
	}
	return seconds
}

func marshalOpenAIUpstreamJSON(value any) ([]byte, error) {
	return json.Marshal(value)
}

func countOpenAIResponseImageOutputsFromJSONBytes(body []byte) int {
	return extractOpenAIImagesBillableCountFromJSONBytes(body)
}

func collectOpenAIResponseImageOutputSizesFromJSONBytes(body []byte) []string {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return nil
	}
	var sizes []string
	for _, path := range []string{"data", "output"} {
		items := gjson.GetBytes(body, path)
		if !items.Exists() || !items.IsArray() {
			continue
		}
		for _, item := range items.Array() {
			if size := strings.TrimSpace(item.Get("size").String()); size != "" {
				sizes = append(sizes, normalizeOpenAIImageSizeTier(size))
			}
		}
	}
	return sizes
}

func (s *OpenAIGatewayService) updateGrokUsageSnapshot(ctx context.Context, accountID int64, snapshot *xai.QuotaSnapshot) {
	if s == nil || s.accountRepo == nil || accountID <= 0 || snapshot == nil {
		return
	}
	if s.codexSnapshotThrottle != nil && !s.codexSnapshotThrottle.Allow(accountID, time.Now()) {
		return
	}
	_ = s.accountRepo.UpdateExtra(ctx, accountID, map[string]any{grokQuotaSnapshotExtraKey: snapshot})
}

func (s *OpenAIGatewayService) handleGrokAccountUpstreamError(ctx context.Context, account *Account, statusCode int, headers http.Header, _ []byte) {
	if s == nil || account == nil {
		return
	}
	switch statusCode {
	case http.StatusUnauthorized:
		s.tempUnscheduleGrok(ctx, account, 10*time.Minute, "grok oauth token unauthorized")
	case http.StatusPaymentRequired:
		s.tempUnscheduleGrok(ctx, account, grokPaymentRequiredCooldown, "grok payment required")
	case http.StatusForbidden:
		s.tempUnscheduleGrok(ctx, account, 30*time.Minute, "grok entitlement or subscription tier denied")
	case http.StatusTooManyRequests:
		cooldown := 2 * time.Minute
		if snapshot := xai.ParseQuotaHeaders(headers, statusCode); snapshot != nil && snapshot.RetryAfterSeconds != nil && *snapshot.RetryAfterSeconds > 0 {
			cooldown = time.Duration(*snapshot.RetryAfterSeconds) * time.Second
		}
		s.tempUnscheduleGrok(ctx, account, cooldown, "grok rate limited")
	default:
		if statusCode >= 500 {
			s.tempUnscheduleGrok(ctx, account, 2*time.Minute, "grok upstream temporary error")
		}
	}
}

func (s *OpenAIGatewayService) tempUnscheduleGrok(ctx context.Context, account *Account, cooldown time.Duration, reason string) {
	if s == nil || account == nil {
		return
	}
	until := time.Now().Add(cooldown)
	if account.TempUnschedulableUntil != nil && account.TempUnschedulableUntil.After(until) {
		until = *account.TempUnschedulableUntil
	}
	account.TempUnschedulableUntil = &until
	account.TempUnschedulableReason = reason
	account.Schedulable = false
	if s.accountRepo == nil {
		return
	}
	_ = s.accountRepo.SetTempUnschedulable(ctx, account.ID, until, reason)
}
