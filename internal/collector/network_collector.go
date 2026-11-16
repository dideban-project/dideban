package collector

import (
	"context"
	"time"

	"github.com/dideban-project/dideban/internal/model"
)

// NetworkCollector exposes network interface traffic metrics.
type NetworkCollector struct {
	readNetDev func() ([]NetInterface, error)
}

var _ Collector = NetworkCollector{}

func (NetworkCollector) Name() string { return "network" }

func (c NetworkCollector) Collect(ctx context.Context) ([]model.Metric, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	read := c.readNetDev
	if read == nil {
		read = ReadNetDev
	}

	ifaces, err := read()
	if err != nil {
		return nil, err
	}

	return networkMetrics(ifaces, time.Now()), nil
}

func networkMetrics(ifaces []NetInterface, ts time.Time) []model.Metric {
	metrics := make([]model.Metric, 0, 2*len(ifaces))

	for _, n := range ifaces {
		metrics = append(metrics,
			model.Metric{
				Name:   "net_rx_bytes",
				Value:  float64(n.RxBytes),
				Time:   ts,
				Labels: map[string]string{"iface": n.Name},
			},
			model.Metric{
				Name:   "net_tx_bytes",
				Value:  float64(n.TxBytes),
				Time:   ts,
				Labels: map[string]string{"iface": n.Name},
			},
		)
	}

	return metrics
}
