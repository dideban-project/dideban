//go:build linux

package collector

import (
	"path/filepath"
	"testing"
)

func TestReadDiskLinux(t *testing.T) {
	got, err := ReadDisk(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.TotalBytes == 0 {
		t.Error("TotalBytes = 0, want a positive size")
	}
	if got.UsedBytes > got.TotalBytes {
		t.Errorf("UsedBytes (%d) > TotalBytes (%d)", got.UsedBytes, got.TotalBytes)
	}
}

func TestReadDiskLinuxMissingPath(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	got, err := ReadDisk(missing)
	if err == nil {
		t.Fatalf("expected error for a missing path, got %+v", got)
	}
	if got != (Disk{}) {
		t.Errorf("got %+v on error, want the zero value", got)
	}
}
