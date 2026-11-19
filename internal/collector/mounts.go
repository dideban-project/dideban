package collector

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// mountFields is the number of fields in every /proc/mounts entry:
// device, mount point, filesystem type, options, dump and pass.
const mountFields = 6

// diskFSTypes lists the filesystem types the disk collector reports.
// Virtual and temporary filesystems (proc, sysfs, tmpfs, devtmpfs, ...) and
// every other type are skipped.
var diskFSTypes = map[string]bool{
	"ext4":  true,
	"xfs":   true,
	"btrfs": true,
}

// Mount describes one mounted filesystem from /proc/mounts.
type Mount struct {
	Device     string `json:"device"`
	MountPoint string `json:"mount_point"`
	FSType     string `json:"fs_type"`
	Options    string `json:"options"`
	Dump       int    `json:"dump"`
	Pass       int    `json:"pass"`
}

// ParseMounts parses the contents of /proc/mounts and returns the mounts whose
// filesystem type is ext4, xfs or btrfs, in input order.
//
// Each entry has the form "<device> <mountpoint> <fstype> <options> <dump> <pass>".
// An entry of a supported type must have exactly six fields, and dump and pass
// must be non-negative integers; anything else is an error. Entries of any
// other filesystem type are skipped without looking at the rest of the line.
// A line with fewer than three fields is an error, because its filesystem type
// cannot be determined. Blank lines are ignored, and input without any entry is
// an error.
//
// Spaces, tabs, newlines and backslashes in the device, mount point and options
// are escaped by the kernel as octal sequences (for example "\040") and are
// decoded here. If a mount point appears more than once (a filesystem mounted
// over another), the last entry wins, so each mount point is reported once.
//
// It takes an io.Reader (not a file path) so it can be tested with plain strings.
func ParseMounts(r io.Reader) ([]Mount, error) {
	var mounts []Mount
	index := make(map[string]int) // mount point -> position in mounts
	entries := 0
	lineNo := 0

	sc := bufio.NewScanner(r)
	for sc.Scan() {
		lineNo++
		fields := strings.Fields(sc.Text())
		if len(fields) == 0 {
			continue
		}
		entries++
		if len(fields) < 3 {
			return nil, fmt.Errorf("mounts: line %d: expected %d fields, got %d", lineNo, mountFields, len(fields))
		}
		if !diskFSTypes[fields[2]] {
			continue
		}

		m, err := parseMount(fields)
		if err != nil {
			return nil, fmt.Errorf("mounts: line %d: %w", lineNo, err)
		}
		if i, ok := index[m.MountPoint]; ok {
			mounts[i] = m
			continue
		}
		index[m.MountPoint] = len(mounts)
		mounts = append(mounts, m)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("mounts: reading: %w", err)
	}
	if entries == 0 {
		return nil, fmt.Errorf("mounts: empty input")
	}

	return mounts, nil
}

// parseMount converts the fields of one supported /proc/mounts entry into a Mount.
func parseMount(fields []string) (Mount, error) {
	if len(fields) != mountFields {
		return Mount{}, fmt.Errorf("%s: expected %d fields, got %d", fields[1], mountFields, len(fields))
	}

	dump, err := parseMountNumber("dump", fields[4])
	if err != nil {
		return Mount{}, fmt.Errorf("%s: %w", fields[1], err)
	}
	pass, err := parseMountNumber("pass", fields[5])
	if err != nil {
		return Mount{}, fmt.Errorf("%s: %w", fields[1], err)
	}

	return Mount{
		Device:     unescapeMountField(fields[0]),
		MountPoint: unescapeMountField(fields[1]),
		FSType:     fields[2],
		Options:    unescapeMountField(fields[3]),
		Dump:       dump,
		Pass:       pass,
	}, nil
}

// parseMountNumber parses the dump or pass field, which must be a non-negative integer.
func parseMountNumber(name, s string) (int, error) {
	v, err := strconv.ParseUint(s, 10, 31)
	if err != nil {
		return 0, fmt.Errorf("parsing %s: %w", name, err)
	}
	return int(v), nil
}

// unescapeMountField decodes the octal escapes (a backslash followed by three
// octal digits, such as "\040" for a space) that the kernel uses in /proc/mounts.
// Anything that is not a valid escape is kept as it is.
func unescapeMountField(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}

	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+3 < len(s) && isOctal(s[i+1]) && isOctal(s[i+2]) && isOctal(s[i+3]) {
			v := int(s[i+1]-'0')<<6 | int(s[i+2]-'0')<<3 | int(s[i+3]-'0')
			if v <= 0xFF {
				b.WriteByte(byte(v))
				i += 3
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func isOctal(c byte) bool { return c >= '0' && c <= '7' }

// ReadMounts reads the real (ext4, xfs, btrfs) mounts from /proc/mounts.
func ReadMounts() ([]Mount, error) {
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return nil, fmt.Errorf("mounts: %w", err)
	}
	defer f.Close()
	return ParseMounts(f)
}
