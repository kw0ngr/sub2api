//go:build unit

package service

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type glmRateLimitCooldownRepo struct {
	mockAccountRepoForGemini
	resetAt time.Time
	calls   int
}

func (r *glmRateLimitCooldownRepo) SetRateLimited(ctx context.Context, id int64, resetAt time.Time) error {
	r.calls++
	r.resetAt = resetAt
	return nil
}

func TestRateLimitService_GLMGeneric429_capsLongRetryAfter(t *testing.T) {
	// Given
	repo := &glmRateLimitCooldownRepo{}
	svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	account := &Account{ID: 1790, Platform: PlatformGLM, Type: AccountTypeAPIKey}
	headers := http.Header{"Retry-After": []string{"18000"}}
	body := []byte(`{"type":"error","error":{"type":"rate_limit_error","message":"You've reached the request rate limit for your account. Visit your dashboard to verify your usage and adjust your plan limits."}}`)

	// When
	before := time.Now()
	svc.handleAPIKeyTemporaryCooldown(context.Background(), account, http.StatusTooManyRequests, headers, body)
	after := time.Now()

	// Then
	require.Equal(t, 1, repo.calls)
	require.GreaterOrEqual(t, repo.resetAt.Sub(before), 14*time.Minute)
	require.LessOrEqual(t, repo.resetAt.Sub(after), 16*time.Minute)
}

func TestRateLimitService_GLMResettableQuota429_honorsRetryAfter(t *testing.T) {
	// Given
	repo := &glmRateLimitCooldownRepo{}
	svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	account := &Account{ID: 1791, Platform: PlatformGLM, Type: AccountTypeAPIKey}
	headers := http.Header{"Retry-After": []string{"7200"}}
	body := []byte(`{"error":{"code":"1310","message":"usage limit will reset later"}}`)

	// When
	before := time.Now()
	svc.handleAPIKeyTemporaryCooldown(context.Background(), account, http.StatusTooManyRequests, headers, body)
	after := time.Now()

	// Then
	require.Equal(t, 1, repo.calls)
	require.GreaterOrEqual(t, repo.resetAt.Sub(before), 119*time.Minute)
	require.LessOrEqual(t, repo.resetAt.Sub(after), 121*time.Minute)
}
