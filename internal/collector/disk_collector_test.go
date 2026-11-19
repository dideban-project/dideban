package collector

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestDiskCollectorName(t *testing.T) {
	if got := (DiskCollector{}).Name(); got != "disk" {
		t.Errorf("Name() = %q, want %q", got, "disk")
	}
}

func TestDiskMetrics(t *testing.T) {
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	got := diskMetrics("/data", Disk{TotalBytes: 4096000, UsedBytes: 3072000}, ts)

	want := []struct {
		name  string
		value float64
	}{
		{"disk_total_bytes", 4096000},
		{"disk_used_bytes", 3072000},
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
		if len(m.Labels) != 1 || m.Labels["mount"] != "/data" {
			t.Errorf("metric %s: labels = %v, want only mount=/data", w.name, m.Labels)
		}
	}
}

func TestDiskMetricsDoNotShareLabelMaps(t *testing.T) {
	got := diskMetrics("/data", Disk{TotalBytes: 2, UsedBytes: 1}, time.Now())
	if len(got) != 2 {
		t.Fatalf("got %d metrics, want 2", len(got))
	}

	got[0].Labels["mount"] = "changed"
	if got[1].Labels["mount"] != "/data" {
		t.Errorf("modifying one metric's labels changed another: %v", got[1].Labels)
	}
}

func TestDiskMetricsMountWithSpecialCharacters(t *testing.T) {
	got := diskMetrics("/mnt/my disk\t\\x", Disk{TotalBytes: 1, UsedBytes: 1}, time.Now())
	for _, m := range got {
		if m.Labels["mount"] != "/mnt/my disk\t\\x" {
			t.Errorf("metric %s: mount label = %q, want the mount point unchanged", m.Name, m.Labels["mount"])
		}
	}
}

func TestCollectDisk(t *testing.T) {
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	mounts := []Mount{
		{Device: "/dev/sda1", MountPoint: "/", FSType: "ext4"},
		{Device: "/dev/sdb1", MountPoint: "/data", FSType: "xfs"},
		{Device: "/dev/sdc1", MountPoint: "/home", FSType: "btrfs"},
	}
	disks := map[string]Disk{
		"/":     {TotalBytes: 1000, UsedBytes: 400},
		"/data": {TotalBytes: 5000, UsedBytes: 5000},
		"/home": {TotalBytes: 0, UsedBytes: 0},
	}
	var asked []string
	stat := func(path string) (Disk, error) {
		asked = append(asked, path)
		return disks[path], nil
	}

	got, err := collectDisk(context.Background(), mounts, stat, ts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The statistics function receives each mount point, in order.
	if strings.Join(asked, ",") != "/,/data,/home" {
		t.Errorf("stat called for %v, want [/ /data /home]", asked)
	}

	want := []struct {
		name  string
		mount string
		value float64
	}{
		{"disk_total_bytes", "/", 1000},
		{"disk_used_bytes", "/", 400},
		{"disk_total_bytes", "/data", 5000},
		{"disk_used_bytes", "/data", 5000},
		{"disk_total_bytes", "/home", 0},
		{"disk_used_bytes", "/home", 0},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d metrics, want %d", len(got), len(want))
	}
	for i, w := range want {
		m := got[i]
		if m.Name != w.name || m.Value != w.value || m.Labels["mount"] != w.mount {
			t.Errorf("metric %d = %+v, want name=%s mount=%s value=%v", i, m, w.name, w.mount, w.value)
		}
		// One consistent timestamp for the whole collection.
		if !m.Time.Equal(ts) {
			t.Errorf("metric %d: time = %v, want %v", i, m.Time, ts)
		}
	}
}

func TestCollectDiskNoMounts(t *testing.T) {
	stat := func(string) (Disk, error) {
		t.Fatal("stat must not be called without mounts")
		return Disk{}, nil
	}

	got, err := collectDisk(context.Background(), nil, stat, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d metrics, want 0", len(got))
	}
}

func TestCollectDiskStatError(t *testing.T) {
	sentinel := errors.New("statfs failed")
	mounts := []Mount{
		{Device: "/dev/sda1", MountPoint: "/", FSType: "ext4"},
		{Device: "/dev/sdb1", MountPoint: "/data", FSType: "xfs"},
	}
	stat := func(path string) (Disk, error) {
		if path == "/data" {
			return Disk{}, sentinel
		}
		return Disk{TotalBytes: 10, UsedBytes: 5}, nil
	}

	got, err := collectDisk(context.Background(), mounts, stat, time.Now())
	if err == nil {
		t.Fatalf("expected error, got %+v", got)
	}
	if got != nil {
		t.Errorf("got %+v on error, want nil", got)
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("error %v does not wrap the stat error", err)
	}
	if !strings.Contains(err.Error(), "/data") {
		t.Errorf("error %q does not name the failing mount", err)
	}
}

func TestCollectDiskCancelledBetweenMounts(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	mounts := []Mount{
		{Device: "/dev/sda1", MountPoint: "/", FSType: "ext4"},
		{Device: "/dev/sdb1", MountPoint: "/data", FSType: "xfs"},
	}
	calls := 0
	stat := func(string) (Disk, error) {
		calls++
		cancel() // cancelled while the first mount is being read
		return Disk{TotalBytes: 10, UsedBytes: 5}, nil
	}

	got, err := collectDisk(ctx, mounts, stat, time.Now())
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
	if got != nil {
		t.Errorf("got %+v, want nil", got)
	}
	if calls != 1 {
		t.Errorf("stat called %d times, want 1 (second mount must not be read)", calls)
	}
}

func TestCollectDiskAlreadyCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	stat := func(string) (Disk, error) {
		t.Fatal("stat must not be called with a cancelled context")
		return Disk{}, nil
	}
	got, err := collectDisk(ctx, []Mount{{MountPoint: "/"}}, stat, time.Now())
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
	if got != nil {
		t.Errorf("got %+v, want nil", got)
	}
}

func TestDiskCollectorCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	metrics, err := (DiskCollector{}).Collect(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
	if metrics != nil {
		t.Errorf("metrics = %v, want nil", metrics)
	}
}
