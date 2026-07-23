package service

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"go.uber.org/zap"
)

const (
	defaultOpenAIStreamFailureThreshold  = 2
	defaultOpenAIStreamFailureWindow     = time.Minute
	defaultOpenAIAccountFailureWindow    = 5 * time.Minute
	defaultOpenAIStreamQuarantineTTL     = 10 * time.Minute
	defaultOpenAIStreamCircuitMaxEntries = 4096
)

type openAIStreamCircuitScope uint8

const (
	openAIStreamCircuitScopeProxy openAIStreamCircuitScope = iota + 1
	openAIStreamCircuitScopeAccount
)

func (s openAIStreamCircuitScope) String() string {
	switch s {
	case openAIStreamCircuitScopeProxy:
		return "proxy"
	case openAIStreamCircuitScopeAccount:
		return "account"
	default:
		return "unknown"
	}
}

type openAIStreamCircuitKey struct {
	scope openAIStreamCircuitScope
	id    int64
}

type openAIStreamCircuitSettings struct {
	failureThreshold int
	failureWindow    time.Duration
	accountWindow    time.Duration
	quarantineTTL    time.Duration
	maxEntries       int
}

type openAIStreamCircuitEntry struct {
	failureCount int
	windowStart  time.Time
	blockedUntil time.Time
	lastTouched  time.Time
}

// openAIStreamCircuit is a bounded, in-process circuit for incomplete
// OpenAI-compatible SSE streams. Shared proxies are isolated by proxy ID;
// accounts without a proxy are isolated by account ID. Restarting the service
// clears observations and every quarantine expires automatically.
type openAIStreamCircuit struct {
	mu       sync.Mutex
	settings openAIStreamCircuitSettings
	entries  map[openAIStreamCircuitKey]openAIStreamCircuitEntry
}

func resolveOpenAIStreamCircuitSettings(s *OpenAIGatewayService) openAIStreamCircuitSettings {
	settings := openAIStreamCircuitSettings{
		failureThreshold: defaultOpenAIStreamFailureThreshold,
		failureWindow:    defaultOpenAIStreamFailureWindow,
		accountWindow:    defaultOpenAIAccountFailureWindow,
		quarantineTTL:    defaultOpenAIStreamQuarantineTTL,
		maxEntries:       defaultOpenAIStreamCircuitMaxEntries,
	}
	if s == nil || s.cfg == nil {
		return settings
	}
	cfg := s.cfg.Gateway.OpenAIProxyStreamCircuit
	if cfg.FailureThreshold > 0 {
		settings.failureThreshold = cfg.FailureThreshold
	}
	if cfg.WindowSeconds > 0 {
		settings.failureWindow = time.Duration(cfg.WindowSeconds) * time.Second
	}
	if cfg.AccountWindowSeconds > 0 {
		settings.accountWindow = time.Duration(cfg.AccountWindowSeconds) * time.Second
	}
	if cfg.TTLSeconds > 0 {
		settings.quarantineTTL = time.Duration(cfg.TTLSeconds) * time.Second
	}
	return settings
}

func newOpenAIStreamCircuit(settings openAIStreamCircuitSettings) *openAIStreamCircuit {
	if settings.failureThreshold <= 0 {
		settings.failureThreshold = defaultOpenAIStreamFailureThreshold
	}
	if settings.failureWindow <= 0 {
		settings.failureWindow = defaultOpenAIStreamFailureWindow
	}
	if settings.accountWindow <= 0 {
		settings.accountWindow = defaultOpenAIAccountFailureWindow
	}
	if settings.quarantineTTL <= 0 {
		settings.quarantineTTL = defaultOpenAIStreamQuarantineTTL
	}
	if settings.maxEntries <= 0 {
		settings.maxEntries = defaultOpenAIStreamCircuitMaxEntries
	}
	return &openAIStreamCircuit{
		settings: settings,
		entries:  make(map[openAIStreamCircuitKey]openAIStreamCircuitEntry),
	}
}

func (s *OpenAIGatewayService) getOpenAIStreamCircuit() *openAIStreamCircuit {
	if s == nil {
		return nil
	}
	s.openaiStreamCircuitOnce.Do(func() {
		if s.openaiStreamCircuit == nil {
			s.openaiStreamCircuit = newOpenAIStreamCircuit(resolveOpenAIStreamCircuitSettings(s))
		}
	})
	return s.openaiStreamCircuit
}

func (c *openAIStreamCircuit) recordFailure(key openAIStreamCircuitKey, now time.Time) (bool, time.Time) {
	if c == nil || key.id <= 0 || key.scope == 0 {
		return false, time.Time{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, exists := c.entries[key]
	if exists && now.Before(entry.blockedUntil) {
		entry.lastTouched = now
		c.entries[key] = entry
		return false, entry.blockedUntil
	}
	if !exists {
		c.ensureCapacityLocked(now)
	}
	failureWindow := c.failureWindowForKey(key)
	if entry.windowStart.IsZero() || now.Before(entry.windowStart) || now.Sub(entry.windowStart) > failureWindow {
		entry.failureCount = 0
		entry.windowStart = now
		entry.blockedUntil = time.Time{}
	}
	entry.failureCount++
	entry.lastTouched = now
	tripped := entry.failureCount >= c.settings.failureThreshold
	if tripped {
		entry.blockedUntil = now.Add(c.settings.quarantineTTL)
	}
	c.entries[key] = entry
	return tripped, entry.blockedUntil
}

func (c *openAIStreamCircuit) recordSuccess(key openAIStreamCircuitKey) bool {
	if c == nil || key.id <= 0 || key.scope == 0 {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.entries[key]; !ok {
		return false
	}
	delete(c.entries, key)
	return true
}

func (c *openAIStreamCircuit) isBlocked(key openAIStreamCircuitKey, now time.Time) bool {
	if c == nil || key.id <= 0 || key.scope == 0 {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok || entry.blockedUntil.IsZero() {
		return false
	}
	if !now.Before(entry.blockedUntil) {
		delete(c.entries, key)
		return false
	}
	return true
}

func (c *openAIStreamCircuit) ensureCapacityLocked(now time.Time) {
	if len(c.entries) < c.settings.maxEntries {
		return
	}
	for key, entry := range c.entries {
		staleObservation := entry.blockedUntil.IsZero() && now.Sub(entry.lastTouched) > c.failureWindowForKey(key)
		expiredQuarantine := !entry.blockedUntil.IsZero() && !now.Before(entry.blockedUntil)
		if staleObservation || expiredQuarantine {
			delete(c.entries, key)
		}
	}
	if len(c.entries) < c.settings.maxEntries {
		return
	}
	var oldestKey openAIStreamCircuitKey
	var oldest time.Time
	for key, entry := range c.entries {
		if oldestKey.id == 0 || entry.lastTouched.Before(oldest) {
			oldestKey = key
			oldest = entry.lastTouched
		}
	}
	if oldestKey.id > 0 {
		delete(c.entries, oldestKey)
	}
}

func (c *openAIStreamCircuit) failureWindowForKey(key openAIStreamCircuitKey) time.Duration {
	if c != nil && key.scope == openAIStreamCircuitScopeAccount {
		return c.settings.accountWindow
	}
	if c != nil {
		return c.settings.failureWindow
	}
	return defaultOpenAIStreamFailureWindow
}

func openAIStreamCircuitTarget(account *Account) (openAIStreamCircuitKey, bool) {
	if account == nil || account.ID <= 0 {
		return openAIStreamCircuitKey{}, false
	}
	if account.ProxyID != nil && *account.ProxyID > 0 {
		return openAIStreamCircuitKey{scope: openAIStreamCircuitScopeProxy, id: *account.ProxyID}, true
	}

	eligible := IsOpenAIChatCompletionsCompatiblePlatform(account.Platform)
	if account.Platform == PlatformGLM {
		eligible = account.IsGLMOpenAICompatible()
	}
	if !eligible {
		return openAIStreamCircuitKey{}, false
	}
	return openAIStreamCircuitKey{scope: openAIStreamCircuitScopeAccount, id: account.ID}, true
}

func (s *OpenAIGatewayService) recordOpenAIStreamDisconnect(account *Account, streamErr error, upstreamRequestID string) {
	target, ok := openAIStreamCircuitTarget(account)
	if !ok || streamErr == nil || errors.Is(streamErr, context.Canceled) || errors.Is(streamErr, context.DeadlineExceeded) {
		return
	}
	circuit := s.getOpenAIStreamCircuit()
	tripped, until := circuit.recordFailure(target, time.Now())
	if !tripped {
		return
	}
	logger.L().With(zap.String("component", "service.openai_gateway")).Warn(
		"openai.stream_target_quarantined_after_disconnect",
		zap.String("circuit_scope", target.scope.String()),
		zap.Int64("circuit_target_id", target.id),
		zap.Int64("account_id", account.ID),
		zap.Time("until", until),
		zap.String("upstream_request_id", upstreamRequestID),
		zap.String("error", sanitizeUpstreamErrorMessage(streamErr.Error())),
	)
}

func (s *OpenAIGatewayService) clearOpenAIStreamDisconnect(account *Account) {
	target, ok := openAIStreamCircuitTarget(account)
	if !ok {
		return
	}
	if circuit := s.getOpenAIStreamCircuit(); circuit != nil {
		circuit.recordSuccess(target)
	}
}

func (s *OpenAIGatewayService) isOpenAIStreamQuarantined(account *Account) bool {
	target, ok := openAIStreamCircuitTarget(account)
	if !ok {
		return false
	}
	circuit := s.getOpenAIStreamCircuit()
	return circuit != nil && circuit.isBlocked(target, time.Now())
}
