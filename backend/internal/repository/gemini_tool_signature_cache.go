package repository

import (
	"context"
	"time"
)

func geminiToolSignatureRedisKey(scope, call string) string {
	return "gemini_tool_signature:" + scope + ":" + call
}

func (c *gatewayCache) GetGeminiToolSignatures(ctx context.Context, scope string, calls []string) (map[string]string, error) {
	out := make(map[string]string)
	if len(calls) == 0 {
		return out, nil
	}
	keys := make([]string, len(calls))
	for i, call := range calls {
		keys[i] = geminiToolSignatureRedisKey(scope, call)
	}
	values, err := c.rdb.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}
	for i, value := range values {
		if signature, ok := value.(string); ok && signature != "" {
			out[calls[i]] = signature
		}
	}
	return out, nil
}

func (c *gatewayCache) PutGeminiToolSignatures(ctx context.Context, scope string, values map[string]string, ttl time.Duration) error {
	pipe := c.rdb.Pipeline()
	for call, signature := range values {
		pipe.Set(ctx, geminiToolSignatureRedisKey(scope, call), signature, ttl)
	}
	_, err := pipe.Exec(ctx)
	return err
}
