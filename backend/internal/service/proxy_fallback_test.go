package service

import (
	"testing"
	"time"
)

func TestResolveProxyFallbackTarget(t *testing.T) {
	now := time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)
	expiredAt := now.Add(-time.Hour)
	future := now.Add(time.Hour)
	id2 := int64(2)
	id3 := int64(3)

	tests := []struct {
		name       string
		start      Proxy
		byID       map[int64]Proxy
		wantChange bool
		wantTarget *int64
	}{
		{
			name:       "direct fallback clears proxy",
			start:      Proxy{ID: 1, FallbackMode: FallbackModeDirect},
			wantChange: true,
		},
		{
			name:       "proxy fallback selects healthy backup",
			start:      Proxy{ID: 1, FallbackMode: FallbackModeProxy, BackupProxyID: &id2},
			byID:       map[int64]Proxy{2: {ID: 2, Status: StatusActive, ExpiresAt: &future}},
			wantChange: true,
			wantTarget: &id2,
		},
		{
			name:       "proxy chain skips expired backup and follows direct",
			start:      Proxy{ID: 1, FallbackMode: FallbackModeProxy, BackupProxyID: &id2},
			byID:       map[int64]Proxy{2: {ID: 2, Status: StatusActive, ExpiresAt: &expiredAt, FallbackMode: FallbackModeDirect}},
			wantChange: true,
		},
		{
			name:       "proxy chain skips expired backup and selects next proxy",
			start:      Proxy{ID: 1, FallbackMode: FallbackModeProxy, BackupProxyID: &id2},
			byID:       map[int64]Proxy{2: {ID: 2, Status: StatusExpired, FallbackMode: FallbackModeProxy, BackupProxyID: &id3}, 3: {ID: 3, Status: StatusActive}},
			wantChange: true,
			wantTarget: &id3,
		},
		{
			name:       "cycle is unresolved",
			start:      Proxy{ID: 1, FallbackMode: FallbackModeProxy, BackupProxyID: &id2},
			byID:       map[int64]Proxy{2: {ID: 2, Status: StatusExpired, FallbackMode: FallbackModeProxy, BackupProxyID: proxyFallbackInt64(1)}},
			wantChange: false,
		},
		{
			name:       "missing backup is unresolved",
			start:      Proxy{ID: 1, FallbackMode: FallbackModeProxy, BackupProxyID: &id2},
			byID:       map[int64]Proxy{},
			wantChange: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTarget, gotChange := ResolveProxyFallbackTarget(tt.start, tt.byID, now)
			if gotChange != tt.wantChange {
				t.Fatalf("change mismatch: got %v want %v", gotChange, tt.wantChange)
			}
			if !sameInt64Ptr(gotTarget, tt.wantTarget) {
				t.Fatalf("target mismatch: got %v want %v", gotTarget, tt.wantTarget)
			}
		})
	}
}

func proxyFallbackInt64(v int64) *int64 {
	return &v
}

func sameInt64Ptr(a, b *int64) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
