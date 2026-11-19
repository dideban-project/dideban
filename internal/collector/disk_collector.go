package collector

import (
	"context"
	"fmt"
	"time"

	"github.com/dideban-project/dideban/internal/model"
)

// DiskCollector exposes the usage of the real mounted filesystems (ext4, xfs,
// btrfs) through the Collector interface.
type DiskCollector struct{}

var _ Collector = DiskCollector{}

// Name implements Collector.
func (DiskCollector) Name() string { return "disk" }

// Collect implements Collector.
func (DiskCollector) Collect(ctx context.Context) ([]model.Metric, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	mounts, err := ReadMounts()
	if err != nil {
		return nil, err
	}
	return collectDisk(ctx, mounts, ReadDisk, time.Now())
}

// collectDisk reads the statistics of every mount with stat and converts them
// into metrics, all stamped with ts. The context is checked before each mount,
// because statfs can be slow. If any mount fails, the whole collection fails
// with an error that names the mount.
//
// It takes the mounts and the statistics function as parameters, so it can be
// tested on every platform without /proc or statfs.
func collectDisk(ctx context.Context, mounts []Mount, stat func(path string) (Disk, error), ts time.Time) ([]model.Metric, error) {
	metrics := make([]model.Metric, 0, 2*len(mounts))
	for _, m := range mounts {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		d, err := stat(m.MountPoint)
		if err != nil {
			return nil, fmt.Errorf("disk: mount %s: %w", m.MountPoint, err)
		}
		metrics = append(metrics, diskMetrics(m.MountPoint, d, ts)...)
	}
	return metrics, nil
}

// diskMetrics converts the statistics of one mount into disk_total_bytes and
// disk_used_bytes, both labeled with the mount point.
func diskMetrics(mount string, d Disk, ts time.Time) []model.Metric {
	return []model.Metric{
		{
			Name:   "disk_total_bytes",
			Value:  float64(d.TotalBytes),
			Time:   ts,
			Labels: map[string]string{"mount": mount},
		},
		{
			Name:   "disk_used_bytes",
			Value:  float64(d.UsedBytes),
			Time:   ts,
			Labels: map[string]string{"mount": mount},
		},
	}
}
