//go:build unit

package service

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type failingDecryptEncryptor struct{}

func (f failingDecryptEncryptor) Encrypt(plaintext string) (string, error) {
	return plaintext, nil
}

func (f failingDecryptEncryptor) Decrypt(string) (string, error) {
	return "", errors.New("decrypt failed")
}

type fakeChannelMonitorRepo struct {
	monitor       *ChannelMonitor
	enabled       []*ChannelMonitor
	insertHistory int
	markChecked   int
}

func (r *fakeChannelMonitorRepo) Create(context.Context, *ChannelMonitor) error {
	return nil
}

func (r *fakeChannelMonitorRepo) GetByID(context.Context, int64) (*ChannelMonitor, error) {
	return cloneChannelMonitor(r.monitor), nil
}

func (r *fakeChannelMonitorRepo) Update(context.Context, *ChannelMonitor) error {
	return nil
}

func (r *fakeChannelMonitorRepo) Delete(context.Context, int64) error {
	return nil
}

func (r *fakeChannelMonitorRepo) List(context.Context, ChannelMonitorListParams) ([]*ChannelMonitor, int64, error) {
	return nil, 0, nil
}

func (r *fakeChannelMonitorRepo) ListEnabled(context.Context) ([]*ChannelMonitor, error) {
	out := make([]*ChannelMonitor, 0, len(r.enabled))
	for _, m := range r.enabled {
		out = append(out, cloneChannelMonitor(m))
	}
	return out, nil
}

func (r *fakeChannelMonitorRepo) MarkChecked(context.Context, int64, time.Time) error {
	r.markChecked++
	return nil
}

func (r *fakeChannelMonitorRepo) InsertHistoryBatch(context.Context, []*ChannelMonitorHistoryRow) error {
	r.insertHistory++
	return nil
}

func (r *fakeChannelMonitorRepo) DeleteHistoryBefore(context.Context, time.Time) (int64, error) {
	return 0, nil
}

func (r *fakeChannelMonitorRepo) ListHistory(context.Context, int64, string, int) ([]*ChannelMonitorHistoryEntry, error) {
	return nil, nil
}

func (r *fakeChannelMonitorRepo) ListLatestPerModel(context.Context, int64) ([]*ChannelMonitorLatest, error) {
	return nil, nil
}

func (r *fakeChannelMonitorRepo) ComputeAvailability(context.Context, int64, int) ([]*ChannelMonitorAvailability, error) {
	return nil, nil
}

func (r *fakeChannelMonitorRepo) ListLatestForMonitorIDs(context.Context, []int64) (map[int64][]*ChannelMonitorLatest, error) {
	return nil, nil
}

func (r *fakeChannelMonitorRepo) ComputeAvailabilityForMonitors(context.Context, []int64, int) (map[int64][]*ChannelMonitorAvailability, error) {
	return nil, nil
}

func (r *fakeChannelMonitorRepo) ListRecentHistoryForMonitors(context.Context, []int64, map[int64]string, int) (map[int64][]*ChannelMonitorHistoryEntry, error) {
	return nil, nil
}

func (r *fakeChannelMonitorRepo) UpsertDailyRollupsFor(context.Context, time.Time) (int64, error) {
	return 0, nil
}

func (r *fakeChannelMonitorRepo) DeleteRollupsBefore(context.Context, time.Time) (int64, error) {
	return 0, nil
}

func (r *fakeChannelMonitorRepo) LoadAggregationWatermark(context.Context) (*time.Time, error) {
	return nil, nil
}

func (r *fakeChannelMonitorRepo) UpdateAggregationWatermark(context.Context, time.Time) error {
	return nil
}

func TestChannelMonitorRunCheckDecryptFailurePreservesError(t *testing.T) {
	// Given
	repo := &fakeChannelMonitorRepo{
		monitor: &ChannelMonitor{
			ID:           7,
			Name:         "stale",
			APIKey:       "stale-ciphertext",
			PrimaryModel: "glm-4.5",
		},
	}
	svc := NewChannelMonitorService(repo, failingDecryptEncryptor{})

	// When
	results, err := svc.RunCheck(context.Background(), 7)

	// Then
	if !errors.Is(err, ErrChannelMonitorAPIKeyDecryptFailed) {
		t.Fatalf("expected ErrChannelMonitorAPIKeyDecryptFailed, got %v", err)
	}
	if reason := infraerrors.Reason(err); reason != "CHANNEL_MONITOR_KEY_DECRYPT_FAILED" {
		t.Fatalf("expected CHANNEL_MONITOR_KEY_DECRYPT_FAILED, got %q", reason)
	}
	if results != nil {
		t.Fatalf("expected no results on decrypt failure, got %#v", results)
	}
	if repo.insertHistory != 0 || repo.markChecked != 0 {
		t.Fatalf("decrypt failure must not persist check results, insert=%d mark=%d", repo.insertHistory, repo.markChecked)
	}
}

func TestListEnabledMonitors_DecryptFailureClassifiesMonitor(t *testing.T) {
	// Given
	repo := &fakeChannelMonitorRepo{
		enabled: []*ChannelMonitor{{
			ID:     8,
			Name:   "stale",
			APIKey: "stale-ciphertext",
		}},
	}
	svc := NewChannelMonitorService(repo, failingDecryptEncryptor{})

	// When
	monitors, err := svc.ListEnabledMonitors(context.Background())

	// Then
	if err != nil {
		t.Fatalf("ListEnabledMonitors returned error: %v", err)
	}
	if len(monitors) != 1 {
		t.Fatalf("expected one monitor, got %d", len(monitors))
	}
	if !monitors[0].APIKeyDecryptFailed {
		t.Fatal("expected decrypt failure flag to be set")
	}
	if monitors[0].APIKey != "" {
		t.Fatal("decrypt failure must clear API key material")
	}
}

func TestListEnabledMonitors_EmptyAPIKeyDoesNotClassifyDecryptFailure(t *testing.T) {
	// Given
	repo := &fakeChannelMonitorRepo{
		enabled: []*ChannelMonitor{{ID: 9, Name: "empty", APIKey: ""}},
	}
	svc := NewChannelMonitorService(repo, failingDecryptEncryptor{})

	// When
	monitors, err := svc.ListEnabledMonitors(context.Background())

	// Then
	if err != nil {
		t.Fatalf("ListEnabledMonitors returned error: %v", err)
	}
	if len(monitors) != 1 {
		t.Fatalf("expected one monitor, got %d", len(monitors))
	}
	if monitors[0].APIKeyDecryptFailed {
		t.Fatal("empty API key should not be classified as decrypt failure")
	}
}

func TestRunOne_DecryptFailureDoesNotEmitDuplicateWarning(t *testing.T) {
	// Given
	var logs bytes.Buffer
	restore := captureSlogForTest(t, &logs)
	defer restore()

	svc := &stubMonitorSvc{runErr: ErrChannelMonitorAPIKeyDecryptFailed}
	r := newRunnerForTest(svc)
	if !r.tryAcquireInFlight(4) {
		t.Fatal("expected in-flight acquire to succeed")
	}

	// When
	r.runOne(4, "broken")

	// Then
	got := logs.String()
	if strings.Contains(got, "channel_monitor: run check failed") {
		t.Fatalf("decrypt failure should not emit generic run-check warning, got logs:\n%s", got)
	}
	if strings.Contains(got, "CHANNEL_MONITOR_KEY_DECRYPT_FAILED") {
		t.Fatalf("runner should leave decrypt-failure logging to decrypt classification, got logs:\n%s", got)
	}
}

func TestSchedule_DecryptFailedMonitorIsNotScheduled(t *testing.T) {
	// Given
	svc := &stubMonitorSvc{runCalled: make(chan int64, 1)}
	r := newRunnerForTest(svc)
	r.Start()

	// When
	r.Schedule(&ChannelMonitor{
		ID:                  4,
		Name:                "broken",
		Enabled:             true,
		IntervalSeconds:     60,
		APIKeyDecryptFailed: true,
	})

	// Then
	if got := runnerTaskCount(r); got != 0 {
		t.Fatalf("decrypt-failed monitor should not be scheduled, got %d tasks", got)
	}

	stoppedWithin(t, r, 3*time.Second)
}

func cloneChannelMonitor(m *ChannelMonitor) *ChannelMonitor {
	if m == nil {
		return nil
	}
	clone := *m
	clone.ExtraModels = append([]string(nil), m.ExtraModels...)
	if m.ExtraHeaders != nil {
		clone.ExtraHeaders = make(map[string]string, len(m.ExtraHeaders))
		for k, v := range m.ExtraHeaders {
			clone.ExtraHeaders[k] = v
		}
	}
	if m.BodyOverride != nil {
		clone.BodyOverride = make(map[string]any, len(m.BodyOverride))
		for k, v := range m.BodyOverride {
			clone.BodyOverride[k] = v
		}
	}
	return &clone
}

func captureSlogForTest(t *testing.T, buf *bytes.Buffer) func() {
	t.Helper()
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	return func() {
		slog.SetDefault(previous)
	}
}
