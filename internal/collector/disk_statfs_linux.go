//go:build linux

package collector

import (
	"fmt"
	"syscall"
)

// ReadDisk returns the capacity and usage of the filesystem mounted at path,
// using statfs(2).
func ReadDisk(path string) (Disk, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return Disk{}, fmt.Errorf("statfs: %w", err)
	}
	// The size fields are signed or unsigned depending on the architecture;
	// int64 holds every variant, and diskFromStatfs rejects invalid values.
	return diskFromStatfs(uint64(st.Blocks), uint64(st.Bfree), int64(st.Frsize), int64(st.Bsize))
}
