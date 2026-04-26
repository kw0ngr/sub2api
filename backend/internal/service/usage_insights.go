package service

import (
	"context"
	"math"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
)

func roundedShare(part, total int64) float64 {
	if total <= 0 || part <= 0 {
		return 0
	}
	return math.Round((float64(part)/float64(total))*10000) / 10000
}

// BuildUsageInsightSummary derives dashboard insights from existing model and
// project aggregates. It intentionally avoids another SQL query.
func BuildUsageInsightSummary(models []usagestats.ModelStat, projects []usagestats.ProjectStat) *usagestats.UsageInsightSummary {
	out := &usagestats.UsageInsightSummary{
		ModelCount:   len(models),
		ProjectCount: len(projects),
	}

	for _, model := range models {
		out.Requests += model.Requests
		out.InputTokens += model.InputTokens
		out.OutputTokens += model.OutputTokens
		out.CacheCreationTokens += model.CacheCreationTokens
		out.CacheReadTokens += model.CacheReadTokens
		out.TotalTokens += model.TotalTokens
		if model.TotalTokens > out.TopModelTokens ||
			(model.TotalTokens == out.TopModelTokens && model.Requests > 0 && out.TopModel == "") {
			out.TopModel = model.Model
			out.TopModelTokens = model.TotalTokens
		}
	}
	out.CacheTokens = out.CacheCreationTokens + out.CacheReadTokens
	out.InputShare = roundedShare(out.InputTokens, out.TotalTokens)
	out.OutputShare = roundedShare(out.OutputTokens, out.TotalTokens)
	out.CacheCreationShare = roundedShare(out.CacheCreationTokens, out.TotalTokens)
	out.CacheReadShare = roundedShare(out.CacheReadTokens, out.TotalTokens)
	out.CacheShare = roundedShare(out.CacheTokens, out.TotalTokens)
	out.TopModelShare = roundedShare(out.TopModelTokens, out.TotalTokens)

	for _, project := range projects {
		if project.TotalTokens > out.TopProjectTokens ||
			(project.TotalTokens == out.TopProjectTokens && project.Requests > 0 && out.TopProjectKey == "") {
			out.TopProjectKey = project.ProjectKey
			out.TopProjectLabel = project.ProjectLabel
			out.TopProjectTokens = project.TotalTokens
		}
	}
	out.TopProjectShare = roundedShare(out.TopProjectTokens, out.TotalTokens)
	return out
}

// GetUserUsageInsights returns TokenArena-style composition and focus metrics.
func (s *UsageService) GetUserUsageInsights(ctx context.Context, userID int64, startTime, endTime time.Time) (*usagestats.UsageInsightSummary, error) {
	models, err := s.GetUserModelStats(ctx, userID, startTime, endTime)
	if err != nil {
		return nil, err
	}
	projects, err := s.GetUserProjectStats(ctx, userID, startTime, endTime)
	if err != nil {
		return nil, err
	}
	return BuildUsageInsightSummary(models, projects), nil
}

// BuildHourlyActivityHeatmap converts hourly trend points into a fixed 7x24
// grid so the frontend can render stable cells even for empty hours.
func BuildHourlyActivityHeatmap(trend []usagestats.TrendDataPoint, loc *time.Location) []usagestats.HourlyActivityHeatmapCell {
	if loc == nil {
		loc = timezone.Location()
	}

	cells := make([]usagestats.HourlyActivityHeatmapCell, 7*24)
	for weekday := 0; weekday < 7; weekday++ {
		for hour := 0; hour < 24; hour++ {
			cells[weekday*24+hour] = usagestats.HourlyActivityHeatmapCell{
				Weekday: weekday,
				Hour:    hour,
			}
		}
	}

	for _, point := range trend {
		t, err := time.ParseInLocation("2006-01-02 15:04", point.Date, loc)
		if err != nil {
			continue
		}
		weekday := int(t.Weekday())
		hour := t.Hour()
		if weekday < 0 || weekday > 6 || hour < 0 || hour > 23 {
			continue
		}
		cell := &cells[weekday*24+hour]
		cell.Requests += point.Requests
		cell.InputTokens += point.InputTokens
		cell.OutputTokens += point.OutputTokens
		cell.CacheCreationTokens += point.CacheCreationTokens
		cell.CacheReadTokens += point.CacheReadTokens
		cell.TotalTokens += point.TotalTokens
		cell.Cost += point.Cost
		cell.ActualCost += point.ActualCost
	}

	return cells
}

func usageDashboardLocation(userTZ string) *time.Location {
	if userTZ != "" {
		if loc, err := time.LoadLocation(userTZ); err == nil {
			return loc
		}
	}
	return timezone.Location()
}

// GetUserHourlyActivity returns hourly activity grouped into local weekday/hour cells.
func (s *UsageService) GetUserHourlyActivity(ctx context.Context, userID int64, startTime, endTime time.Time, userTZ string) ([]usagestats.HourlyActivityHeatmapCell, error) {
	trend, err := s.GetUserUsageTrendByUserID(ctx, userID, startTime, endTime, "hour")
	if err != nil {
		return nil, err
	}
	return BuildHourlyActivityHeatmap(trend, usageDashboardLocation(userTZ)), nil
}
