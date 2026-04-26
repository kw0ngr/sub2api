package service

import (
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/stretchr/testify/require"
)

func TestBuildUsageInsightSummaryComputesMixAndFocus(t *testing.T) {
	insight := BuildUsageInsightSummary(
		[]usagestats.ModelStat{
			{
				Model:               "gpt-5.1",
				Requests:            3,
				InputTokens:         100,
				OutputTokens:        50,
				CacheCreationTokens: 25,
				CacheReadTokens:     25,
				TotalTokens:         200,
			},
			{
				Model:        "claude-sonnet-4.5",
				Requests:     1,
				InputTokens:  80,
				OutputTokens: 20,
				TotalTokens:  100,
			},
		},
		[]usagestats.ProjectStat{
			{
				ProjectKey:   "p1",
				ProjectLabel: "internal-tools",
				Requests:     3,
				TotalTokens:  240,
			},
			{
				ProjectKey:   "p2",
				ProjectLabel: "docs",
				Requests:     1,
				TotalTokens:  60,
			},
		},
	)

	require.Equal(t, int64(4), insight.Requests)
	require.Equal(t, int64(300), insight.TotalTokens)
	require.Equal(t, int64(50), insight.CacheTokens)
	require.Equal(t, 0.6, insight.InputShare)
	require.Equal(t, 0.1667, insight.CacheShare)
	require.Equal(t, "gpt-5.1", insight.TopModel)
	require.Equal(t, 0.6667, insight.TopModelShare)
	require.Equal(t, "internal-tools", insight.TopProjectLabel)
	require.Equal(t, 0.8, insight.TopProjectShare)
	require.Equal(t, 2, insight.ModelCount)
	require.Equal(t, 2, insight.ProjectCount)
}

func TestBuildHourlyActivityHeatmapReturnsFullGrid(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)

	cells := BuildHourlyActivityHeatmap([]usagestats.TrendDataPoint{
		{
			Date:        "2026-04-26 09:00",
			Requests:    2,
			InputTokens: 100,
			TotalTokens: 150,
		},
		{
			Date:         "2026-04-27 21:00",
			Requests:     1,
			OutputTokens: 50,
			TotalTokens:  50,
		},
	}, loc)

	require.Len(t, cells, 168)
	sundayNine := cells[0*24+9]
	require.Equal(t, 0, sundayNine.Weekday)
	require.Equal(t, 9, sundayNine.Hour)
	require.Equal(t, int64(2), sundayNine.Requests)
	require.Equal(t, int64(150), sundayNine.TotalTokens)

	mondayTwentyOne := cells[1*24+21]
	require.Equal(t, 1, mondayTwentyOne.Weekday)
	require.Equal(t, 21, mondayTwentyOne.Hour)
	require.Equal(t, int64(1), mondayTwentyOne.Requests)
	require.Equal(t, int64(50), mondayTwentyOne.OutputTokens)
}
