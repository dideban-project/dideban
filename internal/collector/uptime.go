package collector

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
)

// ParseUptime parses the contents of /proc/uptime and returns the system
// uptime in seconds.
//
// The expected format is a single line such as:
//
//	350735.47 234388.90
//
// Only the first field (seconds since boot) is used; the second field
// (aggregate idle time) is ignored.
// It takes an io.Reader (not a file path) so it can be tested with plain strings.
func ParseUptime(r io.Reader) (float64, error) {
	sc := bufio.NewScanner(r)
	if !sc.Scan() {
		if err := sc.Err(); err != nil {
			return 0, fmt.Errorf("uptime: reading: %w", err)
		}
		return 0, fmt.Errorf("uptime: empty input")
	}
	first := sc.Text()

	// Read the rest of the input so that a reader error after the first line
	// is reported instead of silently ignored. The remaining content is
	// discarded; only the first line is parsed.
	for sc.Scan() {
	}
	if err := sc.Err(); err != nil {
		return 0, fmt.Errorf("uptime: reading: %w", err)
	}

	fields := strings.Fields(first)
	if len(fields) == 0 {
		return 0, fmt.Errorf("uptime: no fields in input")
	}

	v, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, fmt.Errorf("uptime: parsing seconds: %w", err)
	}
	if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
		return 0, fmt.Errorf("uptime: invalid value %q", fields[0])
	}
	return v, nil
}

// ReadUptime reads the current system uptime in seconds from /proc/uptime.
func ReadUptime() (float64, error) {
	f, err := os.Open("/proc/uptime")
	if err != nil {
		return 0, fmt.Errorf("uptime: %w", err)
	}
	defer f.Close()
	return ParseUptime(f)
}
