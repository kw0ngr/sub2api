package service

import (
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func buildReplayTurnPayload(turn, itemBytes int) []byte {
	var b strings.Builder
	_, _ = b.WriteString(`{"type":"response.create","model":"gpt-5.5","stream":true,"input":[`)
	filler := strings.Repeat("x", itemBytes)
	for i := 1; i <= turn; i++ {
		if i > 1 {
			_, _ = b.WriteString(",")
		}
		_, _ = fmt.Fprintf(&b, `{"type":"input_text","text":"turn-%d-%s"}`, i, filler)
	}
	_, _ = b.WriteString(`]}`)
	return []byte(b.String())
}

func TestOpenAIWSReplayStateBuildAllocationBounded(t *testing.T) {
	const (
		turns     = 128
		itemBytes = 10 * 1024
	)

	// Given
	payloads := make([][]byte, 0, turns)
	for turn := 1; turn <= turns; turn++ {
		payloads = append(payloads, buildReplayTurnPayload(turn, itemBytes))
	}
	var history []json.RawMessage
	historyExists := false
	delta := []json.RawMessage{json.RawMessage(`{"type":"function_call","id":"item_1","call_id":"call_1","name":"exec","arguments":"{}"}`)}

	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	// When
	for turn := 1; turn <= turns; turn++ {
		items, exists, err := buildOpenAIWSReplayInputSequence(history, historyExists, payloads[turn-1], turn > 1)
		require.NoError(t, err)
		require.True(t, exists)
		require.Len(t, items, turn)
		history = append(items[:len(items):len(items)], delta...)
		historyExists = true
		history = history[:len(history)-1]
	}

	// Then
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	allocated := after.TotalAlloc - before.TotalAlloc
	t.Logf("allocated=%d bytes", allocated)
	const maxAllocatedBytes = 32 * 1024 * 1024
	require.Lessf(t, allocated, uint64(maxAllocatedBytes), "allocated=%d bytes", allocated)
}
