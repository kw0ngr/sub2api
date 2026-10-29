package repository

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestGeminiToolSignatureRedisIsolationAndExpiry(t *testing.T) {
	addr := os.Getenv("SUB2API_TEST_SIGNATURE_REDIS_ADDR")
	if addr == "" {
		t.Skip("set SUB2API_TEST_SIGNATURE_REDIS_ADDR to an isolated Redis test instance")
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() { _ = rdb.Close() })
	cache := NewGatewayCache(rdb).(service.GeminiToolSignatureCache)
	ctx := context.Background()
	scope := "signature-test-" + time.Now().Format("150405.000000000")
	require.NoError(t, cache.PutGeminiToolSignatures(ctx, scope, map[string]string{"call-digest": "real-signature"}, 150*time.Millisecond))
	// A new repository instance can read the value (no in-process cache dependency).
	second := NewGatewayCache(rdb).(service.GeminiToolSignatureCache)
	got, err := second.GetGeminiToolSignatures(ctx, scope, []string{"call-digest", "missing"})
	require.NoError(t, err)
	require.Equal(t, map[string]string{"call-digest": "real-signature"}, got)
	got, err = second.GetGeminiToolSignatures(ctx, scope+"-other", []string{"call-digest"})
	require.NoError(t, err)
	require.Empty(t, got)
	require.Eventually(t, func() bool {
		got, err := second.GetGeminiToolSignatures(ctx, scope, []string{"call-digest"})
		return err == nil && len(got) == 0
	}, time.Second, 25*time.Millisecond)
}
