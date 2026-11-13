package collector

import (
	"context"
	"time"

	"github.com/dideban-project/dideban/internal/model"
)

// LoadavgCollector exposes ReadLoadavg through the Collector interface.
type LoadavgCollector struct{}

var _ Collector = LoadavgCollector{}

// Name implements Collector.
func (LoadavgCollector) Name() string { return "loadavg" }

// Collect implements Collector.
func (LoadavgCollector) Collect(ctx context.Context) ([]model.Metric, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	l, err := ReadLoadavg()
	if err != nil {
		return nil, err
	}
	return loadavgMetrics(l, time.Now()), nil
}

// loadavgMetrics converts Loadavg into metrics. It is separate from Collect
// so it can be tested without reading /proc.
func loadavgMetrics(l Loadavg, ts time.Time) []model.Metric {
	return []model.Metric{
		{Name: "load1", Value: l.Load1, Time: ts},
		{Name: "load5", Value: l.Load5, Time: ts},
		{Name: "load15", Value: l.Load15, Time: ts},
	}
}
