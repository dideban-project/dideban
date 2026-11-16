package collector

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

const (
	// netDevHeaderLines is the number of header lines at the top of /proc/net/dev.
	netDevHeaderLines = 2
	// netDevRxBytesField is the index of the RX bytes counter after the "iface:" prefix.
	netDevRxBytesField = 0
	// netDevTxBytesField is the index of the TX bytes counter (ninth number) after the "iface:" prefix.
	netDevTxBytesField = 8
)

// NetInterface holds the traffic counters of one network interface in bytes.
type NetInterface struct {
	Name    string `json:"name"`
	RxBytes uint64 `json:"rx_bytes"`
	TxBytes uint64 `json:"tx_bytes"`
}

// ParseNetDev parses the contents of /proc/net/dev.
//
// The first two lines are headers and are skipped. Every other non-blank line
// has the form "<iface>: <16 numbers>"; the first number is the RX bytes and
// the ninth is the TX bytes.
//
// The loopback interface "lo" is skipped before its counters are parsed, so a
// malformed loopback line is ignored. A malformed line for any other interface
// is an error. A line without a ':' is an error, because its interface name
// cannot be determined.
//
// Interfaces are returned in the order they appear in the input.
// It takes an io.Reader (not a file path) so it can be tested with plain strings.
func ParseNetDev(r io.Reader) ([]NetInterface, error) {
	var ifaces []NetInterface
	lines := 0

	sc := bufio.NewScanner(r)
	for sc.Scan() {
		lines++
		if lines <= netDevHeaderLines {
			continue
		}

		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}

		name, counters, err := splitNetDevLine(line)
		if err != nil {
			return nil, fmt.Errorf("netdev: line %d: %w", lines, err)
		}
		if name == "lo" {
			continue
		}

		iface, err := parseNetDevCounters(name, counters)
		if err != nil {
			return nil, fmt.Errorf("netdev: line %d: %w", lines, err)
		}
		ifaces = append(ifaces, iface)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("netdev: reading: %w", err)
	}
	if lines == 0 {
		return nil, fmt.Errorf("netdev: empty input")
	}
	if lines < netDevHeaderLines {
		return nil, fmt.Errorf("netdev: expected %d header lines, got %d", netDevHeaderLines, lines)
	}

	return ifaces, nil
}

// splitNetDevLine splits an interface line at the first ':' into the trimmed
// interface name and the text holding its counters.
func splitNetDevLine(line string) (name, counters string, err error) {
	name, counters, ok := strings.Cut(line, ":")
	if !ok {
		return "", "", fmt.Errorf("missing ':' after interface name")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "", "", fmt.Errorf("empty interface name")
	}
	return name, counters, nil
}

// parseNetDevCounters parses the counters of one interface, e.g.
//
//	4829371622 3871920 0 12 0 0 0 10432 217489033 1982341 0 0 0 0 0 0
//
// The first counter may follow the colon directly (no space) when the RX value
// is large, which is why the text after the colon is passed in on its own.
func parseNetDevCounters(name, counters string) (NetInterface, error) {
	fields := strings.Fields(counters)
	if len(fields) <= netDevTxBytesField {
		return NetInterface{}, fmt.Errorf("%s: expected at least %d fields after interface name, got %d",
			name, netDevTxBytesField+1, len(fields))
	}

	rx, err := strconv.ParseUint(fields[netDevRxBytesField], 10, 64)
	if err != nil {
		return NetInterface{}, fmt.Errorf("%s: parsing rx bytes: %w", name, err)
	}
	tx, err := strconv.ParseUint(fields[netDevTxBytesField], 10, 64)
	if err != nil {
		return NetInterface{}, fmt.Errorf("%s: parsing tx bytes: %w", name, err)
	}

	return NetInterface{Name: name, RxBytes: rx, TxBytes: tx}, nil
}

// ReadNetDev reads the current per-interface traffic counters from /proc/net/dev.
func ReadNetDev() ([]NetInterface, error) {
	f, err := os.Open("/proc/net/dev")
	if err != nil {
		return nil, fmt.Errorf("netdev: %w", err)
	}
	defer f.Close()
	return ParseNetDev(f)
}
