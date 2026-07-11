package service

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
)

type accountUsageCodexProbeRepo struct {
	stubOpenAIAccountRepo
	updateExtraCh chan map[string]any
	rateLimitCh   chan time.Time
}

type grokLocalBudgetUsageRepo struct {
	UsageLogRepository
	windowStats *usagestats.AccountStats
	todayStats  *usagestats.AccountStats
	windowStart time.Time
}

func (r *grokLocalBudgetUsageRepo) GetAccountWindowStats(_ context.Context, _ int64, startTime time.Time) (*usagestats.AccountStats, error) {
	r.windowStart = startTime
	return r.windowStats, nil
}

func (r *grokLocalBudgetUsageRepo) GetAccountTodayStats(_ context.Context, _ int64) (*usagestats.AccountStats, error) {
	return r.todayStats, nil
}

func (r *accountUsageCodexProbeRepo) UpdateExtra(_ context.Context, _ int64, updates map[string]any) error {
	if r.updateExtraCh != nil {
		copied := make(map[string]any, len(updates))
		for k, v := range updates {
			copied[k] = v
		}
		r.updateExtraCh <- copied
	}
	return nil
}

func (r *accountUsageCodexProbeRepo) SetRateLimited(_ context.Context, _ int64, resetAt time.Time) error {
	if r.rateLimitCh != nil {
		r.rateLimitCh <- resetAt
	}
	return nil
}

func TestAccountUsageService_GetGrokUsageBuildsLocal40MinuteBudget(t *testing.T) {
	t.Parallel()

	repo := &grokLocalBudgetUsageRepo{
		windowStats: &usagestats.AccountStats{
			Requests:     29,
			Tokens:       25_800_000,
			Cost:         3.79,
			StandardCost: 3.79,
			UserCost:     3.79,
		},
		todayStats: &usagestats.AccountStats{
			Requests: 204,
			Tokens:   55_000_000,
			Cost:     16.29,
		},
	}
	svc := &AccountUsageService{
		usageLogRepo:     repo,
		grokQuotaFetcher: NewGrokQuotaFetcher(),
	}

	usage, err := svc.getGrokUsage(context.Background(), &Account{
		ID:       42,
		Platform: PlatformGrok,
		Type:     AccountTypeOAuth,
	})
	if err != nil {
		t.Fatalf("getGrokUsage() error = %v", err)
	}
	if usage.GrokLocalTokenBudget == nil {
		t.Fatal("expected local token budget")
	}
	if usage.GrokLocalTokenBudget.WindowMinutes != 40 {
		t.Fatalf("WindowMinutes = %d, want 40", usage.GrokLocalTokenBudget.WindowMinutes)
	}
	if usage.GrokLocalTokenBudget.LimitTokens != 40_000_000 {
		t.Fatalf("LimitTokens = %d, want 40000000", usage.GrokLocalTokenBudget.LimitTokens)
	}
	if usage.GrokLocalTokenBudget.UsedTokens != 25_800_000 {
		t.Fatalf("UsedTokens = %d, want 25800000", usage.GrokLocalTokenBudget.UsedTokens)
	}
	if usage.GrokLocalTokenBudget.RemainingTokens != 14_200_000 {
		t.Fatalf("RemainingTokens = %d, want 14200000", usage.GrokLocalTokenBudget.RemainingTokens)
	}
	if usage.GrokLocalTokenBudget.Utilization < 64.49 || usage.GrokLocalTokenBudget.Utilization > 64.51 {
		t.Fatalf("Utilization = %v, want about 64.5", usage.GrokLocalTokenBudget.Utilization)
	}
	if usage.GrokLocalUsage == nil || usage.GrokLocalUsage.Tokens != 55_000_000 {
		t.Fatalf("GrokLocalUsage = %#v, want today's stats", usage.GrokLocalUsage)
	}
	if age := time.Since(repo.windowStart); age < 39*time.Minute || age > 41*time.Minute {
		t.Fatalf("window start age = %v, want about 40m", age)
	}
}

func TestShouldRefreshOpenAICodexSnapshot(t *testing.T) {
	t.Parallel()

	rateLimitedUntil := time.Now().Add(5 * time.Minute)
	now := time.Now()
	usage := &UsageInfo{
		FiveHour: &UsageProgress{Utilization: 0},
		SevenDay: &UsageProgress{Utilization: 0},
	}

	if !shouldRefreshOpenAICodexSnapshot(&Account{RateLimitResetAt: &rateLimitedUntil}, usage, now) {
		t.Fatal("expected rate-limited account to force codex snapshot refresh")
	}

	if shouldRefreshOpenAICodexSnapshot(&Account{}, usage, now) {
		t.Fatal("expected complete non-rate-limited usage to skip codex snapshot refresh")
	}

	if !shouldRefreshOpenAICodexSnapshot(&Account{}, &UsageInfo{FiveHour: nil, SevenDay: &UsageProgress{}}, now) {
		t.Fatal("expected missing 5h snapshot to require refresh")
	}

	staleAt := now.Add(-(openAIProbeCacheTTL + time.Minute)).Format(time.RFC3339)
	if !shouldRefreshOpenAICodexSnapshot(&Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			"openai_oauth_responses_websockets_v2_enabled": true,
			"codex_usage_updated_at":                       staleAt,
		},
	}, usage, now) {
		t.Fatal("expected stale ws snapshot to trigger refresh")
	}
}

func TestAccountUsageService_ShouldProbeOpenAICodexSnapshotForceBypassesCache(t *testing.T) {
	t.Parallel()

	now := time.Now()
	svc := &AccountUsageService{cache: NewUsageCache()}
	if !svc.shouldProbeOpenAICodexSnapshot(123, now) {
		t.Fatal("first probe should be allowed")
	}
	if svc.shouldProbeOpenAICodexSnapshot(123, now.Add(time.Minute)) {
		t.Fatal("cached probe should be skipped without force")
	}
	if !svc.shouldProbeOpenAICodexSnapshot(123, now.Add(2*time.Minute), true) {
		t.Fatal("manual refresh should bypass probe cache")
	}
}

func TestExtractOpenAICodexProbeUpdatesAccepts429WithCodexHeaders(t *testing.T) {
	t.Parallel()

	headers := make(http.Header)
	headers.Set("x-codex-primary-used-percent", "100")
	headers.Set("x-codex-primary-reset-after-seconds", "604800")
	headers.Set("x-codex-primary-window-minutes", "10080")
	headers.Set("x-codex-secondary-used-percent", "100")
	headers.Set("x-codex-secondary-reset-after-seconds", "18000")
	headers.Set("x-codex-secondary-window-minutes", "300")

	updates, err := extractOpenAICodexProbeUpdates(&http.Response{StatusCode: http.StatusTooManyRequests, Header: headers})
	if err != nil {
		t.Fatalf("extractOpenAICodexProbeUpdates() error = %v", err)
	}
	if len(updates) == 0 {
		t.Fatal("expected codex probe updates from 429 headers")
	}
	if got := updates["codex_5h_used_percent"]; got != 100.0 {
		t.Fatalf("codex_5h_used_percent = %v, want 100", got)
	}
	if got := updates["codex_7d_used_percent"]; got != 100.0 {
		t.Fatalf("codex_7d_used_percent = %v, want 100", got)
	}
}

func TestAccountUsageService_PersistOpenAICodexProbeSnapshotOnlyUpdatesExtra(t *testing.T) {
	t.Parallel()

	repo := &accountUsageCodexProbeRepo{
		updateExtraCh: make(chan map[string]any, 1),
		rateLimitCh:   make(chan time.Time, 1),
	}
	svc := &AccountUsageService{accountRepo: repo}
	svc.persistOpenAICodexProbeSnapshot(321, map[string]any{
		"codex_7d_used_percent": 100.0,
		"codex_7d_reset_at":     time.Now().Add(2 * time.Hour).UTC().Truncate(time.Second).Format(time.RFC3339),
	})

	select {
	case updates := <-repo.updateExtraCh:
		if got := updates["codex_7d_used_percent"]; got != 100.0 {
			t.Fatalf("codex_7d_used_percent = %v, want 100", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("等待 codex 探测快照写入 extra 超时")
	}

	select {
	case got := <-repo.rateLimitCh:
		t.Fatalf("不应将探测快照写入运行时限流状态: %v", got)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestAccountUsageService_GetOpenAIUsage_DoesNotPromoteCodexExtraToRateLimit(t *testing.T) {
	t.Parallel()

	resetAt := time.Now().Add(6 * 24 * time.Hour).UTC().Truncate(time.Second)
	repo := &accountUsageCodexProbeRepo{
		rateLimitCh: make(chan time.Time, 1),
	}
	svc := &AccountUsageService{accountRepo: repo}
	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			"codex_5h_used_percent": 1.0,
			"codex_5h_reset_at":     time.Now().Add(2 * time.Hour).UTC().Truncate(time.Second).Format(time.RFC3339),
			"codex_7d_used_percent": 100.0,
			"codex_7d_reset_at":     resetAt.Format(time.RFC3339),
		},
	}

	usage, err := svc.getOpenAIUsage(context.Background(), account, false)
	if err != nil {
		t.Fatalf("getOpenAIUsage() error = %v", err)
	}
	if usage.SevenDay == nil || usage.SevenDay.Utilization != 100.0 {
		t.Fatalf("预期 7 天用量仍然可见，实际为 %#v", usage.SevenDay)
	}
	if account.RateLimitResetAt != nil {
		t.Fatalf("不应让已耗尽的 codex extra 改写运行时限流状态: %v", account.RateLimitResetAt)
	}
	select {
	case got := <-repo.rateLimitCh:
		t.Fatalf("不应将已耗尽的 codex extra 持久化为运行时限流状态: %v", got)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestBuildCodexUsageProgressFromExtra_ZerosExpiredWindow(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC)

	t.Run("expired 5h window zeroes utilization", func(t *testing.T) {
		extra := map[string]any{
			"codex_5h_used_percent": 42.0,
			"codex_5h_reset_at":     "2026-03-16T10:00:00Z", // 2h ago
		}
		progress := buildCodexUsageProgressFromExtra(extra, "5h", now)
		if progress == nil {
			t.Fatal("expected non-nil progress")
			return
		}
		if progress.Utilization != 0 {
			t.Fatalf("expected Utilization=0 for expired window, got %v", progress.Utilization)
		}
		if progress.RemainingSeconds != 0 {
			t.Fatalf("expected RemainingSeconds=0, got %v", progress.RemainingSeconds)
		}
	})

	t.Run("active 5h window keeps utilization", func(t *testing.T) {
		resetAt := now.Add(2 * time.Hour).Format(time.RFC3339)
		extra := map[string]any{
			"codex_5h_used_percent": 42.0,
			"codex_5h_reset_at":     resetAt,
		}
		progress := buildCodexUsageProgressFromExtra(extra, "5h", now)
		if progress == nil {
			t.Fatal("expected non-nil progress")
			return
		}
		if progress.Utilization != 42.0 {
			t.Fatalf("expected Utilization=42, got %v", progress.Utilization)
		}
	})

	t.Run("expired 7d window zeroes utilization", func(t *testing.T) {
		extra := map[string]any{
			"codex_7d_used_percent": 88.0,
			"codex_7d_reset_at":     "2026-03-15T00:00:00Z", // yesterday
		}
		progress := buildCodexUsageProgressFromExtra(extra, "7d", now)
		if progress == nil {
			t.Fatal("expected non-nil progress")
			return
		}
		if progress.Utilization != 0 {
			t.Fatalf("expected Utilization=0 for expired 7d window, got %v", progress.Utilization)
		}
	})
}

func TestCodexWindowStatsStartUsesResetWindow(t *testing.T) {
	now := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	resetAt := now.Add(2 * time.Hour)

	got := codexWindowStatsStart(&UsageProgress{ResetsAt: &resetAt}, 5*time.Hour, now)
	want := resetAt.Add(-5 * time.Hour)
	if !got.Equal(want) {
		t.Fatalf("stats start = %s, want %s", got, want)
	}

	expiredResetAt := now.Add(-time.Hour)
	got = codexWindowStatsStart(&UsageProgress{ResetsAt: &expiredResetAt}, 5*time.Hour, now)
	want = now.Add(-5 * time.Hour)
	if !got.Equal(want) {
		t.Fatalf("fallback stats start = %s, want %s", got, want)
	}
}
