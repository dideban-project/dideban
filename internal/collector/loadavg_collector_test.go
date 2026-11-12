package collector

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestLoadavgCollectorName(t *testing.T) {
	if got := (LoadavgCollector{}).Name(); got != "loadavg" {
		t.Errorf("Name() = %q, want %q", got, "loadavg")
	}
}

func TestLoadavgMetrics(t *testing.T) {
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	got := loadavgMetrics(Loadavg{Load1: 0.42, Load5: 0.50, Load15: 0.61}, ts)

	want := []struct {
		name  string
		value float64
	}{
		{"load1", 0.42},
		{"load5", 0.50},
		{"load15", 0.61},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d metrics, want %d", len(got), len(want))
	}
	for i, w := range want {
		m := got[i]
		if m.Name != w.name {
			t.Errorf("metric %d: name = %q, want %q", i, m.Name, w.name)
		}
		if m.Value != w.value {
			t.Errorf("metric %s: value = %v, want %v", w.name, m.Value, w.value)
		}
		if !m.Time.Equal(ts) {
			t.Errorf("metric %s: time = %v, want %v", w.name, m.Time, ts)
		}
		if m.Labels != nil {
			t.Errorf("metric %s: labels = %v, want nil", w.name, m.Labels)
		}
	}
}

func TestLoadavgCollectorCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	metrics, err := (LoadavgCollector{}).Collect(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
	if metrics != nil {
		t.Errorf("metrics = %v, want nil", metrics)
	}
}
