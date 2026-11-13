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
	for _, code := range []string{"1308", "1310"} {
		for _, status := range []int{http.StatusTooManyRequests, http.StatusForbidden} {
			t.Run(fmt.Sprintf("%s/%d", code, status), func(t *testing.T) {
				repo := &rateLimitAccountRepoStub{}
				svc := &RateLimitService{accountRepo: repo}
				account := &Account{ID: 1784, Platform: PlatformGLM, Type: AccountTypeAPIKey}
				body := []byte(fmt.Sprintf(`{"error":{"code":%q,"message":"[%s][已达到使用上限。您的限额将在 %s 重置。][request]"}}`, code, code, reset.Format("2006-01-02 15:04:05")))
				require.True(t, isGLMResettableQuotaError(body))
				require.True(t, svc.HandleUpstreamError(context.Background(), account, status, nil, body, "glm-5.3-flash"))
				if status == http.StatusTooManyRequests {
					require.NotNil(t, repo.lastRateLimitResetAt)
					require.True(t, reset.Equal(*repo.lastRateLimitResetAt))
					require.Zero(t, repo.tempCalls)
				} else {
					require.NotNil(t, repo.lastTempUntil)
					require.True(t, reset.Equal(*repo.lastTempUntil))
					require.Zero(t, repo.rateLimitedCalls)
				}
				require.Zero(t, repo.setErrorCalls)
			})
		}
	}
}

func TestParseGLMQuotaResetTimeScopesAndValidation(t *testing.T) {
	now := time.Date(2026, 11, 13, 11, 13, 52, 0, time.UTC)
	for _, tc := range []struct {
		name, code, message string
		want                bool
	}{
		{"published timestamp", "1310", "限额将在 2026-11-13 20:36:35 重置。", true},
		{"1308 production log", "1308", "[1308][已达到 5 小时的使用上限。您的限额将在 2026-11-13 20:36:35 重置。][2026111319135212266e7ac1fa4ec4]", true},
		{"old timestamp", "1310", "限额将在 2026-11-13 19:13:51 重置。", false},
		{"1308 old timestamp", "1308", "限额将在 2026-11-13 19:13:51 重置。", false},
		{"current timestamp", "1310", "限额将在 2026-11-13 19:13:52 重置。", false},
		{"1308 current timestamp", "1308", "限额将在 2026-11-13 19:13:52 重置。", false},
		{"invalid date", "1310", "限额将在 2026-02-30 05:51:54 重置。", false},
		{"1308 invalid date", "1308", "限额将在 2026-02-30 05:51:54 重置。", false},
		{"other error", "1305", "限额将在 2026-11-13 20:36:35 重置。", false},
		{"no date", "1310", "使用上限，请稍后重试", false},
		{"1308 no date", "1308", "使用上限，请稍后重试", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := []byte(fmt.Sprintf(`{"error":{"code":%q,"message":%q}}`, tc.code, tc.message))
			reset := parseGLMQuotaResetTime(body, now)
			if tc.want {
				require.NotNil(t, reset)
				require.Equal(t, "2026-11-13T12:36:35Z", reset.UTC().Format(time.RFC3339))
				_, offset := reset.Zone()
				require.Equal(t, 8*60*60, offset)
			} else {
				require.Nil(t, reset)
			}
		})
	}
}

func TestGLMShortPublishedResetBeatsDefaultButNotLaterRetryAfter(t *testing.T) {
	reset := time.Now().In(time.FixedZone("CST", 8*60*60)).Add(5 * time.Minute).Truncate(time.Second)
	for _, code := range []string{"1308", "1310"} {
		for _, status := range []int{http.StatusTooManyRequests, http.StatusForbidden} {
			for _, retry := range []struct {
				name, header, body string
				later              bool
			}{
				{name: "no retry"},
				{name: "earlier header", header: "60"},
				{name: "later header", header: "7200", later: true},
				{name: "earlier body", body: "60"},
				{name: "later body", body: "7200", later: true},
			} {
				t.Run(fmt.Sprintf("%s/%d/%s", code, status, retry.name), func(t *testing.T) {
					body := []byte(fmt.Sprintf(`{"error":{"code":%q,"message":"限额将在 %s 重置"},"retry-after":%q}`, code, reset.Format("2006-01-02 15:04:05"), retry.body))
					repo := &rateLimitAccountRepoStub{}
					svc := &RateLimitService{accountRepo: repo}
					before := time.Now()
					require.True(t, svc.HandleUpstreamError(context.Background(), &Account{ID: 1, Platform: PlatformGLM, Type: AccountTypeAPIKey},
						status, http.Header{"Retry-After": []string{retry.header}}, body))
					actual := repo.lastRateLimitResetAt
					if status == http.StatusForbidden {
						actual = repo.lastTempUntil
					}
					require.NotNil(t, actual)
					if retry.later {
						require.WithinDuration(t, before.Add(2*time.Hour), *actual, time.Second)
					} else {
						require.True(t, reset.Equal(*actual))
					}
				})
			}
		}
	}
}

func TestGLMExpiredPublishedResetIsNotPersisted(t *testing.T) {
	reset := time.Now().In(time.FixedZone("CST", 8*60*60)).Add(-time.Hour).Truncate(time.Second)
	for _, code := range []string{"1308", "1310"} {
		for _, status := range []int{http.StatusTooManyRequests, http.StatusForbidden} {
			t.Run(fmt.Sprintf("%s/%d", code, status), func(t *testing.T) {
				body := []byte(fmt.Sprintf(`{"error":{"code":%q,"message":"限额将在 %s 重置"}}`, code, reset.Format("2006-01-02 15:04:05")))
				repo := &rateLimitAccountRepoStub{}
				svc := &RateLimitService{accountRepo: repo}
				before := time.Now()
				require.True(t, svc.HandleUpstreamError(context.Background(), &Account{ID: 1, Platform: PlatformGLM, Type: AccountTypeAPIKey},
					status, nil, body))
				actual := repo.lastRateLimitResetAt
				cooldown := apiKeyGLMGeneric429Cooldown
				if status == http.StatusForbidden {
					actual = repo.lastTempUntil
					cooldown = apiKeyServerErrorCooldown
				}
				require.NotNil(t, actual)
				require.True(t, actual.After(before))
				require.WithinDuration(t, before.Add(cooldown), *actual, time.Second)
				require.Zero(t, repo.setErrorCalls)
			})
		}
	}
}
