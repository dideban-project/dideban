//go:build !linux

package collector

import (
	"errors"
	"runtime"
	"strings"
	"testing"
)

func TestReadDiskUnsupported(t *testing.T) {
	got, err := ReadDisk(t.TempDir())
	if err == nil {
		t.Fatalf("expected an error on a non-Linux platform, got %+v", got)
	}
	if !errors.Is(err, errDiskUnsupported) {
		t.Errorf("error %v does not wrap errDiskUnsupported", err)
	}
	if !strings.Contains(err.Error(), runtime.GOOS) {
		t.Errorf("error %q does not name the platform %q", err, runtime.GOOS)
	}
	if got != (Disk{}) {
		t.Errorf("got %+v on error, want the zero value", got)
	}
}
