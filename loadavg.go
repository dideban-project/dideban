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

// LoadAvg holds the system load averages over the last 1, 5 and 15 minutes.
type LoadAvg struct {
	Load1  float64 `json:"load1"`
	Load5  float64 `json:"load5"`
	Load15 float64 `json:"load15"`
}

// ParseLoadavg parses the contents of /proc/loadavg.
//
// The expected format is a single line such as:
//
//	0.20 0.18 0.12 1/80 11206
//
// Only the first three fields (the 1, 5 and 15 minute averages) are used.
// It takes an io.Reader (not a file path) so it can be tested with plain strings.
func ParseLoadavg(r io.Reader) (LoadAvg, error) {
	sc := bufio.NewScanner(r)
	if !sc.Scan() {
		if err := sc.Err(); err != nil {
			return LoadAvg{}, fmt.Errorf("loadavg: reading: %w", err)
		}
		return LoadAvg{}, fmt.Errorf("loadavg: empty input")
	}

	fields := strings.Fields(sc.Text())
	if len(fields) < 3 {
		return LoadAvg{}, fmt.Errorf("loadavg: expected at least 3 fields, got %d", len(fields))
	}

	var values [3]float64
	names := [3]string{"load1", "load5", "load15"}
	for i, name := range names {
		v, err := parseLoadValue(fields[i])
		if err != nil {
			return LoadAvg{}, fmt.Errorf("loadavg: parsing %s: %w", name, err)
		}
		values[i] = v
	}

	return LoadAvg{Load1: values[0], Load5: values[1], Load15: values[2]}, nil
}

// parseLoadValue parses a single load average value and rejects values that
// cannot be a valid load (negative, NaN or infinite).
func parseLoadValue(s string) (float64, error) {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
		return 0, fmt.Errorf("invalid value %q", s)
	}
	return v, nil
}

// ReadLoadavg reads the current load averages from /proc/loadavg.
func ReadLoadavg() (LoadAvg, error) {
	f, err := os.Open("/proc/loadavg")
	if err != nil {
		return LoadAvg{}, fmt.Errorf("loadavg: %w", err)
	}
	defer f.Close()
	return ParseLoadavg(f)
}
