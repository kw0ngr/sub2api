package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpenAIStreamCircuitThresholdTTLAndSuccessReset(t *testing.T) {
	base := time.Unix(1_800_000_000, 0)
	key := openAIStreamCircuitKey{scope: openAIStreamCircuitScopeAccount, id: 2104}
	circuit := newOpenAIStreamCircuit(openAIStreamCircuitSettings{
		failureThreshold: 2,
		failureWindow:    time.Minute,
		accountWindow:    time.Minute,
		quarantineTTL:    10 * time.Minute,
		maxEntries:       16,
	})

	tripped, _ := circuit.recordFailure(key, base)
	require.False(t, tripped)
	require.False(t, circuit.isBlocked(key, base))
	require.True(t, circuit.recordSuccess(key))

	tripped, _ = circuit.recordFailure(key, base.Add(10*time.Second))
	require.False(t, tripped, "a completed stream must clear the previous failure observation")
	tripped, until := circuit.recordFailure(key, base.Add(20*time.Second))
	require.True(t, tripped)
	require.Equal(t, base.Add(20*time.Second+10*time.Minute), until)
	require.True(t, circuit.isBlocked(key, until.Add(-time.Nanosecond)))
	require.False(t, circuit.isBlocked(key, until), "TTL expiry must re-admit the target")

	other := openAIStreamCircuitKey{scope: openAIStreamCircuitScopeAccount, id: 2105}
	tripped, _ = circuit.recordFailure(other, base)
	require.False(t, tripped)
	tripped, _ = circuit.recordFailure(other, base.Add(2*time.Minute))
	require.False(t, tripped, "failures outside the window must not accumulate")
}

func TestOpenAIStreamCircuitBoundsEntries(t *testing.T) {
	base := time.Unix(1_800_000_000, 0)
	circuit := newOpenAIStreamCircuit(openAIStreamCircuitSettings{
		failureThreshold: 1,
		failureWindow:    time.Minute,
		accountWindow:    time.Minute,
		quarantineTTL:    10 * time.Minute,
		maxEntries:       2,
	})
	keys := []openAIStreamCircuitKey{
		{scope: openAIStreamCircuitScopeAccount, id: 1},
		{scope: openAIStreamCircuitScopeAccount, id: 2},
		{scope: openAIStreamCircuitScopeAccount, id: 3},
	}
	for i, key := range keys {
		circuit.recordFailure(key, base.Add(time.Duration(i)*time.Second))
	}

	circuit.mu.Lock()
	defer circuit.mu.Unlock()
	require.Len(t, circuit.entries, 2)
	_, oldestRetained := circuit.entries[keys[0]]
	require.False(t, oldestRetained, "the oldest entry must be evicted at the bound")
}

func TestOpenAIStreamCircuitUsesLongerWindowForUnproxiedLongStreams(t *testing.T) {
	base := time.Unix(1_800_000_000, 0)
	circuit := newOpenAIStreamCircuit(openAIStreamCircuitSettings{
		failureThreshold: 2,
		failureWindow:    time.Minute,
		accountWindow:    5 * time.Minute,
		quarantineTTL:    10 * time.Minute,
		maxEntries:       16,
	})
	accountKey := openAIStreamCircuitKey{scope: openAIStreamCircuitScopeAccount, id: 2104}
	proxyKey := openAIStreamCircuitKey{scope: openAIStreamCircuitScopeProxy, id: 88}

	circuit.recordFailure(accountKey, base)
	tripped, _ := circuit.recordFailure(accountKey, base.Add(75*time.Second))
	require.True(t, tripped, "two 65s-class long-stream failures must quarantine an unproxied account")

	circuit.recordFailure(proxyKey, base)
	tripped, _ = circuit.recordFailure(proxyKey, base.Add(75*time.Second))
	require.False(t, tripped, "shared proxies retain the upstream-compatible 60s observation window")
}

func TestOpenAIStreamCircuitTargetsSharedProxyOrUnproxiedAccount(t *testing.T) {
	proxyID := int64(88)
	proxyA, ok := openAIStreamCircuitTarget(&Account{ID: 1, Platform: PlatformOpenAI, ProxyID: &proxyID})
	require.True(t, ok)
	proxyB, ok := openAIStreamCircuitTarget(&Account{ID: 2, Platform: PlatformGrok, ProxyID: &proxyID})
	require.True(t, ok)
	require.Equal(t, proxyA, proxyB, "accounts sharing a proxy must share one circuit")
	require.Equal(t, openAIStreamCircuitScopeProxy, proxyA.scope)

	grok, ok := openAIStreamCircuitTarget(&Account{ID: 2104, Platform: PlatformGrok})
	require.True(t, ok)
	require.Equal(t, openAIStreamCircuitKey{scope: openAIStreamCircuitScopeAccount, id: 2104}, grok)

	glmAnthropic, ok := openAIStreamCircuitTarget(&Account{
		ID: 3001, Platform: PlatformGLM, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"compat_mode": GLMCompatModeAnthropic},
	})
	require.False(t, ok)
	require.Zero(t, glmAnthropic)
	glmOpenAI, ok := openAIStreamCircuitTarget(&Account{
		ID: 3002, Platform: PlatformGLM, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"compat_mode": GLMCompatModeOpenAI},
	})
	require.True(t, ok)
	require.Equal(t, openAIStreamCircuitScopeAccount, glmOpenAI.scope)
}

type streamReadThenErrorCloser struct {
	reader *strings.Reader
	err    error
}

func (r *streamReadThenErrorCloser) Read(p []byte) (int, error) {
	if r.reader != nil && r.reader.Len() > 0 {
		return r.reader.Read(p)
	}
	return 0, r.err
}

func (r *streamReadThenErrorCloser) Close() error { return nil }

func TestOpenAIStreamingDisconnectQuarantinesUnproxiedGrokAccount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	account := &Account{
		ID: 2104, Name: "third-party-grok", Platform: PlatformGrok,
		Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true,
	}
	svc := &OpenAIGatewayService{
		openaiStreamCircuit: newOpenAIStreamCircuit(openAIStreamCircuitSettings{
			failureThreshold: 2,
			failureWindow:    time.Minute,
			accountWindow:    5 * time.Minute,
			quarantineTTL:    10 * time.Minute,
			maxEntries:       16,
		}),
	}
	readErr := errors.New("stream ID 7; INTERNAL_ERROR; received from peer")
	partial := "data: {\"id\":\"chatcmpl-partial\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"partial\"}}]}\n\n"

	for attempt := 1; attempt <= 2; attempt++ {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Body: &streamReadThenErrorCloser{
				reader: strings.NewReader(partial),
				err:    readErr,
			},
			Header: http.Header{"X-Request-Id": []string{"rid-grok-disconnect"}},
		}

		_, err := svc.handleStreamingResponse(c.Request.Context(), resp, c, account, time.Now(), "grok-4.5", "grok-4.5")
		require.Error(t, err)
		require.Contains(t, recorder.Body.String(), "partial")
		require.Equal(t, attempt == 2, svc.isOpenAIStreamQuarantined(account))
	}
}

func TestOpenAIStreamCircuitIgnoresClientCancellationAndClearsOnTerminal(t *testing.T) {
	account := &Account{ID: 2104, Platform: PlatformGrok}
	svc := &OpenAIGatewayService{openaiStreamCircuit: newOpenAIStreamCircuit(openAIStreamCircuitSettings{
		failureThreshold: 2,
		failureWindow:    time.Minute,
		accountWindow:    5 * time.Minute,
		quarantineTTL:    10 * time.Minute,
		maxEntries:       16,
	})}

	svc.recordOpenAIStreamDisconnect(account, context.Canceled, "")
	svc.recordOpenAIStreamDisconnect(account, context.DeadlineExceeded, "")
	require.False(t, svc.isOpenAIStreamQuarantined(account))

	svc.recordOpenAIStreamDisconnect(account, io.ErrUnexpectedEOF, "")
	svc.clearOpenAIStreamDisconnect(account)
	svc.recordOpenAIStreamDisconnect(account, io.ErrUnexpectedEOF, "")
	require.False(t, svc.isOpenAIStreamQuarantined(account), "terminal success must reset the failure window")
}

func TestOpenAIAccountSchedulerSkipsQuarantinedUnproxiedGrokAccount(t *testing.T) {
	accounts := []Account{
		{ID: 2104, Platform: PlatformGrok, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 0},
		{ID: 2000, Platform: PlatformGrok, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 5},
	}
	svc := &OpenAIGatewayService{
		accountRepo: stubOpenAIAccountRepo{accounts: accounts},
		openaiStreamCircuit: newOpenAIStreamCircuit(openAIStreamCircuitSettings{
			failureThreshold: 1,
			failureWindow:    time.Minute,
			accountWindow:    5 * time.Minute,
			quarantineTTL:    10 * time.Minute,
			maxEntries:       16,
		}),
	}
	target, ok := openAIStreamCircuitTarget(&accounts[0])
	require.True(t, ok)
	svc.openaiStreamCircuit.recordFailure(target, time.Now())

	selection, _, err := svc.SelectAccountWithSchedulerForPlatform(
		context.Background(), PlatformGrok, nil, "", "", "grok-4.5", nil, OpenAIUpstreamTransportAny,
	)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.NotNil(t, selection.Account)
	require.Equal(t, int64(2000), selection.Account.ID)
}
