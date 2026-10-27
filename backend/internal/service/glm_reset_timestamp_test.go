//go:build unit

package service

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGLMResettableQuotaHonorsPublishedResetTimestamp(t *testing.T) {
	zone := time.FixedZone("CST", 8*60*60)
	reset := time.Now().In(zone).Add(26 * time.Hour).Truncate(time.Second)
	for _, status := range []int{http.StatusTooManyRequests, http.StatusForbidden} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			repo := &rateLimitAccountRepoStub{}
			svc := &RateLimitService{accountRepo: repo}
			account := &Account{ID: 1784, Platform: PlatformGLM, Type: AccountTypeAPIKey}
			body := []byte(fmt.Sprintf(`{"error":{"code":"1310","message":"[1310][您已达到每周/每月使用上限，您的限额将在 %s 重置。][request]"}}`, reset.Format("2006-01-02 15:04:05")))
			require.True(t, svc.HandleUpstreamError(context.Background(), account, status, nil, body, "glm-5.3-flash"))
			if status == http.StatusTooManyRequests {
				require.NotNil(t, repo.lastRateLimitResetAt)
				require.True(t, reset.Equal(*repo.lastRateLimitResetAt))
			} else {
				require.NotNil(t, repo.lastTempUntil)
				require.True(t, reset.Equal(*repo.lastTempUntil))
			}
			require.Zero(t, repo.setErrorCalls)
		})
	}
}

func TestParseGLMQuotaResetTimeScopesAndValidation(t *testing.T) {
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name, code, message string
		want                bool
	}{
		{"published timestamp", "1310", "限额将在 2026-10-28 05:51:54 重置。", true},
		{"old timestamp", "1310", "限额将在 2026-10-26 05:51:54 重置。", false},
		{"invalid date", "1310", "限额将在 2026-02-30 05:51:54 重置。", false},
		{"other error", "1305", "限额将在 2026-10-28 05:51:54 重置。", false},
		{"no date", "1310", "使用上限，请稍后重试", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := []byte(fmt.Sprintf(`{"error":{"code":%q,"message":%q}}`, tc.code, tc.message))
			reset := parseGLMQuotaResetTime(body, now)
			if tc.want {
				require.NotNil(t, reset)
				require.Equal(t, "2026-10-27T21:51:54Z", reset.UTC().Format(time.RFC3339))
			} else {
				require.Nil(t, reset)
			}
		})
	}
}

func TestGLMShortPublishedResetBeatsDefaultButNotLaterRetryAfter(t *testing.T) {
	reset := time.Now().In(time.FixedZone("CST", 8*60*60)).Add(5 * time.Minute).Truncate(time.Second)
	body := []byte(fmt.Sprintf(`{"error":{"code":"1310","message":"限额将在 %s 重置"}}`, reset.Format("2006-01-02 15:04:05")))
	for _, code := range []int{429, 403} {
		for _, retry := range []string{"", "7200"} {
			repo := &rateLimitAccountRepoStub{}
			svc := &RateLimitService{accountRepo: repo}
			before := time.Now()
			require.True(t, svc.HandleUpstreamError(context.Background(), &Account{ID: 1, Platform: PlatformGLM, Type: AccountTypeAPIKey},
				code, http.Header{"Retry-After": []string{retry}}, body))
			actual := repo.lastRateLimitResetAt
			if code == 403 {
				actual = repo.lastTempUntil
			}
			require.NotNil(t, actual)
			if retry == "" {
				require.True(t, reset.Equal(*actual))
			} else {
				require.WithinDuration(t, before.Add(2*time.Hour), *actual, time.Second)
			}
		}
	}
}
