//go:build !linux

package collector

import (
	"errors"
	"fmt"
	"runtime"
)

// errDiskUnsupported is returned by ReadDisk on platforms other than Linux.
var errDiskUnsupported = errors.New("disk statistics are only supported on Linux")

// ReadDisk is only implemented on Linux, where it uses statfs(2). On other
// platforms it returns an error, so the package still builds and its
// platform independent code can be tested there.
func ReadDisk(path string) (Disk, error) {
	return Disk{}, fmt.Errorf("%w (running on %s)", errDiskUnsupported, runtime.GOOS)
}
