package collector

import (
	"math"
	"strings"
	"testing"
)

func TestDiskFromStatfs(t *testing.T) {
	tests := []struct {
		name         string
		blocks       uint64
		free         uint64
		fragmentSize int64
		blockSize    int64
		want         Disk
		wantErr      bool
	}{
		{
			name:         "partly used",
			blocks:       1000,
			free:         250,
			fragmentSize: 4096,
			blockSize:    4096,
			want:         Disk{TotalBytes: 4096000, UsedBytes: 3072000},
		},
		{
			name:         "completely free",
			blocks:       1000,
			free:         1000,
			fragmentSize: 4096,
			blockSize:    4096,
			want:         Disk{TotalBytes: 4096000, UsedBytes: 0},
		},
		{
			name:         "completely used",
			blocks:       1000,
			free:         0,
			fragmentSize: 512,
			blockSize:    4096,
			want:         Disk{TotalBytes: 512000, UsedBytes: 512000},
		},
		{
			name:         "large filesystem",
			blocks:       2_000_000_000,
			free:         500_000_000,
			fragmentSize: 4096,
			blockSize:    4096,
			want:         Disk{TotalBytes: 8_192_000_000_000, UsedBytes: 6_144_000_000_000},
		},
		{
			name:         "empty filesystem",
			blocks:       0,
			free:         0,
			fragmentSize: 4096,
			blockSize:    4096,
			want:         Disk{},
		},
		{
			name:         "fragment size is preferred over block size",
			blocks:       100,
			free:         50,
			fragmentSize: 1024,
			blockSize:    65536,
			want:         Disk{TotalBytes: 102400, UsedBytes: 51200},
		},
		{
			name:         "block size is used when fragment size is zero",
			blocks:       100,
			free:         50,
			fragmentSize: 0,
			blockSize:    4096,
			want:         Disk{TotalBytes: 409600, UsedBytes: 204800},
		},
		{
			name:         "invalid block size is ignored when fragment size is valid",
			blocks:       100,
			free:         50,
			fragmentSize: 1024,
			blockSize:    -1,
			want:         Disk{TotalBytes: 102400, UsedBytes: 51200},
		},
		{
			name:         "free blocks exceed total blocks",
			blocks:       100,
			free:         101,
			fragmentSize: 4096,
			blockSize:    4096,
			wantErr:      true,
		},
		{
			name:         "fragment and block size both zero",
			blocks:       100,
			free:         50,
			fragmentSize: 0,
			blockSize:    0,
			wantErr:      true,
		},
		{
			name:         "fragment size zero and block size negative",
			blocks:       100,
			free:         50,
			fragmentSize: 0,
			blockSize:    -4096,
			wantErr:      true,
		},
		{
			name:         "negative fragment size",
			blocks:       100,
			free:         50,
			fragmentSize: -1,
			blockSize:    4096,
			wantErr:      true,
		},
		{
			name:         "largest total that still fits (MaxUint64 bytes)",
			blocks:       math.MaxUint64,
			free:         0,
			fragmentSize: 1,
			blockSize:    1,
			want:         Disk{TotalBytes: math.MaxUint64, UsedBytes: math.MaxUint64},
		},
		{
			name:         "product one below the overflow limit",
			blocks:       1<<32 - 1,
			free:         0,
			fragmentSize: 1<<32 + 1,
			blockSize:    4096,
			want:         Disk{TotalBytes: math.MaxUint64, UsedBytes: math.MaxUint64},
		},
		{
			name:         "huge total that fits, with almost everything free",
			blocks:       1 << 62,
			free:         1<<62 - 1,
			fragmentSize: 3,
			blockSize:    3,
			want:         Disk{TotalBytes: 3 << 62, UsedBytes: 3},
		},
		{
			name:         "largest possible block size that still fits",
			blocks:       2,
			free:         0,
			fragmentSize: math.MaxInt64,
			blockSize:    4096,
			want:         Disk{TotalBytes: math.MaxUint64 - 1, UsedBytes: math.MaxUint64 - 1},
		},
		{
			name:         "total overflows by one (2^64 bytes)",
			blocks:       1 << 32,
			free:         0,
			fragmentSize: 1 << 32,
			blockSize:    4096,
			wantErr:      true,
		},
		{
			name:         "total overflows: max blocks times two",
			blocks:       math.MaxUint64,
			free:         0,
			fragmentSize: 2,
			blockSize:    2,
			wantErr:      true,
		},
		{
			name:         "total overflows with a nearly all-free filesystem",
			blocks:       1 << 63,
			free:         1<<63 - 1,
			fragmentSize: 2,
			blockSize:    2,
			wantErr:      true,
		},
		{
			name:         "total overflows with the largest block size",
			blocks:       3,
			free:         0,
			fragmentSize: math.MaxInt64,
			blockSize:    4096,
			wantErr:      true,
		},
		{
			name:         "total overflows when the block size is the fallback",
			blocks:       1 << 32,
			free:         0,
			fragmentSize: 0,
			blockSize:    1 << 32,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := diskFromStatfs(tt.blocks, tt.free, tt.fragmentSize, tt.blockSize)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %+v", got)
				}
				if got != (Disk{}) {
					t.Errorf("got %+v on error, want the zero value", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestDiskFromStatfsErrorMessages(t *testing.T) {
	tests := []struct {
		name         string
		blocks       uint64
		free         uint64
		fragmentSize int64
		blockSize    int64
		wantContains string
	}{
		{"total overflow", math.MaxUint64, 0, 2, 2, "total bytes overflow"},
		{"free exceeds total", 10, 11, 4096, 4096, "exceed"},
		{"zero block size", 10, 5, 0, 0, "invalid block size"},
		{"negative fragment size", 10, 5, -1, 4096, "invalid fragment size"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := diskFromStatfs(tt.blocks, tt.free, tt.fragmentSize, tt.blockSize)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantContains) {
				t.Errorf("error %q does not contain %q", err, tt.wantContains)
			}
		})
	}
}

func TestStatfsBlockSize(t *testing.T) {
	tests := []struct {
		name         string
		fragmentSize int64
		blockSize    int64
		want         uint64
		wantErr      bool
	}{
		{"fragment size wins", 1024, 4096, 1024, false},
		{"fallback to block size", 0, 4096, 4096, false},
		{"invalid block size ignored", 512, -1, 512, false},
		{"both zero", 0, 0, 0, true},
		{"zero fragment and negative block size", 0, -1, 0, true},
		{"negative fragment size", -1, 4096, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := statfsBlockSize(tt.fragmentSize, tt.blockSize)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %d", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestBlocksToBytes(t *testing.T) {
	tests := []struct {
		name      string
		blocks    uint64
		blockSize uint64
		want      uint64
		wantOK    bool
	}{
		{"zero blocks", 0, 4096, 0, true},
		{"zero block size", 1000, 0, 0, true},
		{"small product", 1000, 4096, 4096000, true},
		{"max times one", math.MaxUint64, 1, math.MaxUint64, true},
		{"one times max", 1, math.MaxUint64, math.MaxUint64, true},
		{"one below the limit", 1<<32 - 1, 1<<32 + 1, math.MaxUint64, true},
		{"2^32 times 2^32", 1 << 32, 1 << 32, 0, false},
		{"max times two", math.MaxUint64, 2, 0, false},
		{"2^63 times two", 1 << 63, 2, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := blocksToBytes(tt.blocks, tt.blockSize)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}
