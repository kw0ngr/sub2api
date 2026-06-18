package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestUsageLogRepositoryGetGlobalStatsSplitsCacheTokens(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := &usageLogRepository{sql: db}
	start := time.Unix(100, 0).UTC()
	end := time.Unix(200, 0).UTC()
	rows := sqlmock.NewRows([]string{
		"total_requests",
		"total_input_tokens",
		"total_output_tokens",
		"total_cache_tokens",
		"total_cache_creation_tokens",
		"total_cache_read_tokens",
		"total_cost",
		"total_actual_cost",
		"avg_duration_ms",
	}).AddRow(int64(2), int64(10), int64(20), int64(7), int64(3), int64(4), 1.2, 1.1, 50.0)
	mock.ExpectQuery("(?s).*total_cache_creation_tokens.*total_cache_read_tokens.*FROM usage_logs.*").
		WithArgs(start, end).
		WillReturnRows(rows)

	stats, err := repo.GetGlobalStats(context.Background(), start, end)
	require.NoError(t, err)
	require.Equal(t, int64(7), stats.TotalCacheTokens)
	require.Equal(t, int64(3), stats.TotalCacheCreationTokens)
	require.Equal(t, int64(4), stats.TotalCacheReadTokens)
	require.Equal(t, int64(37), stats.TotalTokens)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageLogRepositoryGetUserStatsAggregatedSplitsCacheTokens(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := &usageLogRepository{sql: db}
	start := time.Unix(100, 0).UTC()
	end := time.Unix(200, 0).UTC()
	rows := sqlmock.NewRows([]string{
		"total_requests",
		"total_input_tokens",
		"total_output_tokens",
		"total_cache_tokens",
		"total_cache_creation_tokens",
		"total_cache_read_tokens",
		"total_cost",
		"total_actual_cost",
		"avg_duration_ms",
	}).AddRow(int64(1), int64(5), int64(6), int64(9), int64(2), int64(7), 0.5, 0.4, 80.0)
	mock.ExpectQuery("(?s).*total_cache_creation_tokens.*total_cache_read_tokens.*FROM usage_logs.*WHERE user_id = \\$1.*").
		WithArgs(int64(42), start, end).
		WillReturnRows(rows)

	stats, err := repo.GetUserStatsAggregated(context.Background(), 42, start, end)
	require.NoError(t, err)
	require.Equal(t, int64(9), stats.TotalCacheTokens)
	require.Equal(t, int64(2), stats.TotalCacheCreationTokens)
	require.Equal(t, int64(7), stats.TotalCacheReadTokens)
	require.Equal(t, int64(20), stats.TotalTokens)
	require.NoError(t, mock.ExpectationsWereMet())
}
