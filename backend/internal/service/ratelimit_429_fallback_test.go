package service

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type rateLimit429FallbackRepo struct {
	AccountRepository

	calls   int
	lastID  int64
	lastSet time.Time
}

type rateLimit429SettingRepo struct {
	SettingRepository

	data map[string]string
}

func (r *rateLimit429SettingRepo) GetValue(_ context.Context, key string) (string, error) {
	return r.data[key], nil
}

func (r *rateLimit429FallbackRepo) SetRateLimited(_ context.Context, id int64, resetAt time.Time) error {
	r.calls++
	r.lastID = id
	r.lastSet = resetAt
	return nil
}

func TestRateLimitService_Handle429_AnthropicNoResetUsesFallbackCooldown(t *testing.T) {
	repo := &rateLimit429FallbackRepo{}
	settingsRepo := &rateLimit429SettingRepo{data: map[string]string{}}
	data, err := json.Marshal(RateLimit429CooldownSettings{Enabled: true, CooldownSeconds: 12})
	require.NoError(t, err)
	settingsRepo.data[SettingKeyRateLimit429CooldownSettings] = string(data)

	svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	svc.SetSettingService(NewSettingService(settingsRepo, &config.Config{}))
	account := &Account{ID: 45, Platform: PlatformAnthropic, Type: AccountTypeOAuth}

	before := time.Now()
	svc.handle429(context.Background(), account, http.Header{}, []byte(`{"error":{"type":"rate_limit_error","message":"Extra usage required"}}`))
	after := time.Now()

	require.Equal(t, 1, repo.calls)
	require.Equal(t, int64(45), repo.lastID)
	require.False(t, repo.lastSet.Before(before.Add(12*time.Second)))
	require.False(t, repo.lastSet.After(after.Add(12*time.Second)))
}

func TestRateLimitService_Handle429_AnthropicNoResetFallbackDisabledSkipsMark(t *testing.T) {
	repo := &rateLimit429FallbackRepo{}
	settingsRepo := &rateLimit429SettingRepo{data: map[string]string{}}
	data, err := json.Marshal(RateLimit429CooldownSettings{Enabled: false, CooldownSeconds: 12})
	require.NoError(t, err)
	settingsRepo.data[SettingKeyRateLimit429CooldownSettings] = string(data)

	svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	svc.SetSettingService(NewSettingService(settingsRepo, &config.Config{}))
	account := &Account{ID: 46, Platform: PlatformAnthropic, Type: AccountTypeOAuth}

	svc.handle429(context.Background(), account, http.Header{}, []byte(`{"error":{"type":"rate_limit_error","message":"Extra usage required"}}`))

	require.Zero(t, repo.calls)
}
