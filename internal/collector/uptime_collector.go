package collector

import (
	"context"
	"time"

	"github.com/dideban-project/dideban/internal/model"
)

// UptimeCollector exposes ReadUptime through the Collector interface.
type UptimeCollector struct{}

var _ Collector = UptimeCollector{}

// Name implements Collector.
func (UptimeCollector) Name() string { return "uptime" }

// Collect implements Collector.
func (UptimeCollector) Collect(ctx context.Context) ([]model.Metric, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	seconds, err := ReadUptime()
	if err != nil {
		return nil, err
	}
	return uptimeMetrics(seconds, time.Now()), nil
}

// uptimeMetrics converts the uptime in seconds into metrics. It is separate
// from Collect so it can be tested without reading /proc.
func uptimeMetrics(seconds float64, ts time.Time) []model.Metric {
	return []model.Metric{
		{Name: "uptime_seconds", Value: seconds, Time: ts},
	}
}
