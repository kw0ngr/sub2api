package service

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// ProxyExpiryService periodically marks expired proxies and reroutes bound
// accounts according to each proxy fallback policy.
type ProxyExpiryService struct {
	proxyRepo ProxyRepository
	interval  time.Duration
	stopCh    chan struct{}
	stopOnce  sync.Once
	wg        sync.WaitGroup
}

func NewProxyExpiryService(proxyRepo ProxyRepository, interval time.Duration) *ProxyExpiryService {
	return &ProxyExpiryService{
		proxyRepo: proxyRepo,
		interval:  interval,
		stopCh:    make(chan struct{}),
	}
}

func (s *ProxyExpiryService) Start() {
	if s == nil || s.proxyRepo == nil || s.interval <= 0 {
		return
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()
		s.runOnce()
		for {
			select {
			case <-ticker.C:
				s.runOnce()
			case <-s.stopCh:
				return
			}
		}
	}()
}

func (s *ProxyExpiryService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() { close(s.stopCh) })
	s.wg.Wait()
}

func (s *ProxyExpiryService) runOnce() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	changed, err := s.proxyRepo.SweepExpiredProxies(ctx, time.Now().UTC())
	if err != nil {
		slog.Warn("proxy_expiry_sweep_failed", "error", err)
		return
	}
	if changed > 0 {
		slog.Info("proxy_expiry_accounts_rerouted", "count", changed)
	}
}
