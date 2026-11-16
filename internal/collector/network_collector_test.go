package collector

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNetworkCollectorName(t *testing.T) {
	if got := (NetworkCollector{}).Name(); got != "network" {
		t.Errorf("Name() = %q, want %q", got, "network")
	}
}

func TestNetworkMetrics(t *testing.T) {
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	ifaces := []NetInterface{
		{Name: "eth0", RxBytes: 4829371622, TxBytes: 217489033},
		{Name: "wlan0", RxBytes: 91827364, TxBytes: 12093847},
	}

	got := networkMetrics(ifaces, ts)

	want := []struct {
		name  string
		iface string
		value float64
	}{
		{"net_rx_bytes", "eth0", 4829371622},
		{"net_tx_bytes", "eth0", 217489033},
		{"net_rx_bytes", "wlan0", 91827364},
		{"net_tx_bytes", "wlan0", 12093847},
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
			t.Errorf("metric %d: value = %v, want %v", i, m.Value, w.value)
		}
		if !m.Time.Equal(ts) {
			t.Errorf("metric %d: time = %v, want %v", i, m.Time, ts)
		}
		if len(m.Labels) != 1 || m.Labels["iface"] != w.iface {
			t.Errorf("metric %d: labels = %v, want iface=%q", i, m.Labels, w.iface)
		}
	}
}

func TestNetworkMetricsDoNotShareLabelMaps(t *testing.T) {
	got := networkMetrics(
		[]NetInterface{{Name: "eth0", RxBytes: 1, TxBytes: 2}},
		time.Now(),
	)

	got[0].Labels["iface"] = "changed"
	if got[1].Labels["iface"] != "eth0" {
		t.Errorf("modifying one metric's labels changed another")
	}
}

func TestNetworkMetricsNoInterfaces(t *testing.T) {
	if got := networkMetrics(nil, time.Now()); len(got) != 0 {
		t.Errorf("got %d metrics, want 0", len(got))
	}
}

func TestNetworkCollectorCollect(t *testing.T) {
	c := NetworkCollector{
		readNetDev: func() ([]NetInterface, error) {
			return []NetInterface{
				{Name: "eth0", RxBytes: 100, TxBytes: 200},
			}, nil
		},
	}

	before := time.Now()
	metrics, err := c.Collect(context.Background())
	after := time.Now()

	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}
	if len(metrics) != 2 {
		t.Fatalf("got %d metrics, want 2", len(metrics))
	}

	if metrics[0].Name != "net_rx_bytes" || metrics[0].Value != 100 {
		t.Errorf("unexpected RX metric: %+v", metrics[0])
	}
	if metrics[1].Name != "net_tx_bytes" || metrics[1].Value != 200 {
		t.Errorf("unexpected TX metric: %+v", metrics[1])
	}

	for _, m := range metrics {
		if m.Labels["iface"] != "eth0" {
			t.Errorf("unexpected labels: %v", m.Labels)
		}
		if m.Time.Before(before) || m.Time.After(after) {
			t.Errorf("unexpected timestamp: %v", m.Time)
		}
	}
}

func TestNetworkCollectorReadError(t *testing.T) {
	wantErr := errors.New("read error")
	c := NetworkCollector{
		readNetDev: func() ([]NetInterface, error) {
			return nil, wantErr
		},
	}

	metrics, err := c.Collect(context.Background())
	if !errors.Is(err, wantErr) {
		t.Errorf("error = %v, want %v", err, wantErr)
	}
	if metrics != nil {
		t.Errorf("metrics = %v, want nil", metrics)
	}
}

func TestNetworkCollectorCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	metrics, err := (NetworkCollector{}).Collect(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
	if metrics != nil {
		t.Errorf("metrics = %v, want nil", metrics)
	}
}
