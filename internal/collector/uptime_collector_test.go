package collector

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestUptimeCollectorName(t *testing.T) {
	if got := (UptimeCollector{}).Name(); got != "uptime" {
		t.Errorf("Name() = %q, want %q", got, "uptime")
	}
}

func TestUptimeMetrics(t *testing.T) {
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	got := uptimeMetrics(350735.47, ts)

	if len(got) != 1 {
		t.Fatalf("got %d metrics, want 1", len(got))
	}
	m := got[0]
	if m.Name != "uptime_seconds" {
		t.Errorf("name = %q, want %q", m.Name, "uptime_seconds")
	}
	if m.Value != 350735.47 {
		t.Errorf("value = %v, want %v", m.Value, 350735.47)
	}
	if !m.Time.Equal(ts) {
		t.Errorf("time = %v, want %v", m.Time, ts)
	}
	if m.Labels != nil {
		t.Errorf("labels = %v, want nil", m.Labels)
	}
}

func TestUptimeCollectorCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	metrics, err := (UptimeCollector{}).Collect(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
	if metrics != nil {
		t.Errorf("metrics = %v, want nil", metrics)
	}
}
