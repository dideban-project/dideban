package collector

import (
	"fmt"
	"math/bits"
)

// Disk holds the capacity and usage of one filesystem in bytes.
type Disk struct {
	TotalBytes uint64 `json:"total_bytes"`
	UsedBytes  uint64 `json:"used_bytes"`
}

// diskFromStatfs converts the values of a statfs(2) result into a Disk.
//
// Total bytes is blocks times the block size. Used bytes is blocks minus
// freeBlocks, times the block size. freeBlocks must be the statfs Bfree field
// (all free blocks, including the ones reserved for root), which is what df
// reports in its "Used" column. The Bavail field (free blocks available to
// unprivileged users) is deliberately not used: it would count the reserved
// blocks, typically 5% of an ext4 filesystem, as used space.
//
// Block counts are in units of the fragment size (statfs f_frsize). Older
// kernels leave it zero, in which case the block size (f_bsize) is used, as df
// does. The sizes are signed values in the statfs structure on most platforms,
// so they are validated here instead of being converted blindly: a negative
// value, or zero after the fallback, is an error.
//
// An error is also returned if freeBlocks exceeds blocks, or if a byte count
// does not fit in a uint64. It is separate from the platform specific ReadDisk
// so it can be tested on every platform.
func diskFromStatfs(blocks, freeBlocks uint64, fragmentSize, blockSize int64) (Disk, error) {
	size, err := statfsBlockSize(fragmentSize, blockSize)
	if err != nil {
		return Disk{}, err
	}
	if freeBlocks > blocks {
		return Disk{}, fmt.Errorf("free blocks (%d) exceed total blocks (%d)", freeBlocks, blocks)
	}

	total, ok := blocksToBytes(blocks, size)
	if !ok {
		return Disk{}, fmt.Errorf("total bytes overflow uint64 (%d blocks of %d bytes)", blocks, size)
	}
	// Used blocks never exceed total blocks, so this cannot overflow once the
	// total fits; it is checked anyway so the result is never silently wrong.
	usedBlocks := blocks - freeBlocks
	used, ok := blocksToBytes(usedBlocks, size)
	if !ok {
		return Disk{}, fmt.Errorf("used bytes overflow uint64 (%d blocks of %d bytes)", usedBlocks, size)
	}

	return Disk{TotalBytes: total, UsedBytes: used}, nil
}

// statfsBlockSize picks the unit of the statfs block counts: the fragment size
// if it is set, otherwise the block size. The result is always positive.
func statfsBlockSize(fragmentSize, blockSize int64) (uint64, error) {
	if fragmentSize < 0 {
		return 0, fmt.Errorf("invalid fragment size %d", fragmentSize)
	}
	size := fragmentSize
	if size == 0 {
		size = blockSize
	}
	if size <= 0 {
		return 0, fmt.Errorf("invalid block size %d", size)
	}
	return uint64(size), nil
}

// blocksToBytes multiplies a block count by the block size. The boolean is
// false if the product does not fit in a uint64.
func blocksToBytes(blocks, blockSize uint64) (uint64, bool) {
	hi, lo := bits.Mul64(blocks, blockSize)
	return lo, hi == 0
}
