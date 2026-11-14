package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const (
	geminiHeaderWaitTimeout = 60 * time.Second
	geminiHeaderWaitBudget  = 90 * time.Second
	geminiTimeoutBackoff    = 30 * time.Second
	geminiHeaderBudgetKey   = "gemini_header_wait_started"
)

var (
	errGeminiHeaderTimeout = errors.New("Gemini timed out waiting for upstream response headers")
	errGeminiWaitBudget    = errors.New("Gemini upstream response-header wait budget exhausted")
)

type geminiCancelBody struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (body *geminiCancelBody) Close() error {
	defer body.cancel()
	return body.ReadCloser.Close()
}

// The deadline ends at response headers, not at the end of a healthy SSE stream.
func doGeminiWithHeaderTimeout(upstream HTTPUpstream, req *http.Request, proxy string, account *Account, timeout time.Duration) (*http.Response, error) {
	ctx, cancel := context.WithCancel(req.Context())
	var mu sync.Mutex
	finished, timedOut := false, false
	timer := time.AfterFunc(timeout, func() {
		mu.Lock()
		defer mu.Unlock()
		if !finished {
			timedOut = true
			cancel()
		}
	})
	resp, err := upstream.Do(req.Clone(ctx), proxy, account.ID, account.Concurrency)
	mu.Lock()
	finished = true
	expired := timedOut
	mu.Unlock()
	timer.Stop()
	if expired && req.Context().Err() == nil {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		cancel()
		return nil, errGeminiHeaderTimeout
	}
	if err != nil {
		cancel()
		return resp, err
	}
	if err := req.Context().Err(); err != nil {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		cancel()
		return nil, err
	}
	if resp == nil || resp.Body == nil {
		cancel()
		return resp, err
	}
	resp.Body = &geminiCancelBody{ReadCloser: resp.Body, cancel: cancel}
	return resp, nil
}

func (s *OpenAIGatewayService) doOpenAICompatibleUpstream(c *gin.Context, req *http.Request, proxy string, account *Account) (*http.Response, error) {
	if account.Platform != PlatformGemini {
		return s.httpUpstream.Do(req, proxy, account.ID, account.Concurrency)
	}
	wait, budget := geminiHeaderLimits(c)
	if c != nil {
		started, exists := c.Get(geminiHeaderBudgetKey)
		start, ok := started.(time.Time)
		if !exists || !ok {
			start = time.Now()
			c.Set(geminiHeaderBudgetKey, start)
		}
		remaining := budget - time.Since(start)
		if remaining <= 0 {
			return nil, errGeminiWaitBudget
		}
		if remaining < wait {
			wait = remaining
		}
	}
	return doGeminiWithHeaderTimeout(s.httpUpstream, req, proxy, account, wait)
}

func geminiHeaderLimits(c *gin.Context) (time.Duration, time.Duration) {
	if c != nil {
		raw, _ := c.Get(OpsUpstreamRequestBodyKey)
		body, _ := raw.([]byte)
		effort := gjson.GetBytes(body, "reasoning_effort").String()
		if effort == "" {
			effort = gjson.GetBytes(body, "extra_body.google.thinking_config.thinking_level").String()
		}
		if effort == "high" || effort == "xhigh" {
			// High-thinking tool calls can legitimately take over a minute before headers.
			return 90 * time.Second, 120 * time.Second
		}
	}
	return geminiHeaderWaitTimeout, geminiHeaderWaitBudget
}

func (s *OpenAIGatewayService) backoffGeminiTimeout(ctx context.Context, account *Account, model string) {
	if s.accountRepo == nil || model == "" {
		return
	}
	bg, cancel := context.WithTimeout(context.WithoutCancel(ctx), openAIAccountStateUpdateTimeout)
	defer cancel()
	until := time.Now().Add(geminiTimeoutBackoff)
	if old := account.modelRateLimitResetAt(model); old != nil && old.After(until) {
		until = *old
	}
	if err := s.accountRepo.SetModelRateLimit(bg, account.ID, model, until); err != nil {
		slog.Warn("gemini_header_timeout_backoff_failed", "account_id", account.ID, "model", model, "error", err)
		return
	}
	slog.Warn("gemini_header_timeout_model_backoff", "account_id", account.ID, "model", model, "until", until)
}
