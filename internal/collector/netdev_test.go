package collector

import (
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"
)

// netDevHeader is the two-line header found at the top of /proc/net/dev.
const netDevHeader = `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
`

// netDevSandboxSample is a representative sample. It is NOT a verified capture
// from a Dideban development or production host.
//
// Origin, as stated by the author of this test (the repository cannot prove
// it): the text is unmodified output of `cat /proc/net/dev` run in a Linux
// sandbox VM (kernel "Linux 6.18.44-fc-v470 x86_64") on 2026-11-15. Treat it as
// a representative sample. To get a verified real sample, run
// `cat /proc/net/dev` on a real host and replace this constant; the expected
// values in TestParseNetDevSandboxSample must then be updated to match.
const netDevSandboxSample = `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
    lo:       0       0    0    0    0     0          0         0        0       0    0    0    0     0       0          0
  ifb0:       0       0    0    0    0     0          0         0        0       0    0    0    0     0       0          0
  ifb1:       0       0    0    0    0     0          0         0        0       0    0    0    0     0       0          0
  eth0:   17842      34    0    3    0     0          0         0    13967      42    0    0    0     0       0          0
`

// netDevSyntheticSample is NOT captured from any host. It was written by hand
// to follow the /proc/net/dev format, and all its numbers are invented. It
// exists to cover things the sandbox sample lacks: non-zero loopback counters,
// several active interfaces, and large byte counters.
const netDevSyntheticSample = `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
    lo: 1583921   18734    0    0    0     0          0         0  1583921   18734    0    0    0     0       0          0
  eth0: 4829371622 3871920    0   12    0     0          0     10432 217489033 1982341    0    0    0     0       0          0
 wlan0:  91827364  102934    0    0    0     0          0       211  12093847   88120    0    0    0     0       0          0
docker0:    5821      63    0    0    0     0          0         0      9433      71    0    0    0     0       0          0
veth3a1:  120394    1420    0    0    0     0          0         0    883712    1311    0    0    0     0       0          0
`

func sameInterfaces(got, want []NetInterface) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestParseNetDevSandboxSample(t *testing.T) {
	got, err := ParseNetDev(strings.NewReader(netDevSandboxSample))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// lo is skipped; interfaces with all-zero counters are still reported.
	want := []NetInterface{
		{Name: "ifb0", RxBytes: 0, TxBytes: 0},
		{Name: "ifb1", RxBytes: 0, TxBytes: 0},
		{Name: "eth0", RxBytes: 17842, TxBytes: 13967},
	}
	if !sameInterfaces(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestParseNetDevSyntheticSample(t *testing.T) {
	got, err := ParseNetDev(strings.NewReader(netDevSyntheticSample))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []NetInterface{
		{Name: "eth0", RxBytes: 4829371622, TxBytes: 217489033},
		{Name: "wlan0", RxBytes: 91827364, TxBytes: 12093847},
		{Name: "docker0", RxBytes: 5821, TxBytes: 9433},
		{Name: "veth3a1", RxBytes: 120394, TxBytes: 883712},
	}
	if !sameInterfaces(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestParseNetDev(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []NetInterface
		wantErr bool
	}{
		{
			name:  "single interface",
			input: netDevHeader + "  eth0: 100 2 0 0 0 0 0 0 300 4 0 0 0 0 0 0\n",
			want:  []NetInterface{{Name: "eth0", RxBytes: 100, TxBytes: 300}},
		},
		{
			name: "multiple interfaces keep input order",
			input: netDevHeader +
				"eth1: 1 0 0 0 0 0 0 0 2 0 0 0 0 0 0 0\n" +
				"eth0: 3 0 0 0 0 0 0 0 4 0 0 0 0 0 0 0\n",
			want: []NetInterface{
				{Name: "eth1", RxBytes: 1, TxBytes: 2},
				{Name: "eth0", RxBytes: 3, TxBytes: 4},
			},
		},
		{
			name: "rx is the first number and tx is the ninth",
			input: netDevHeader +
				"eth0: 11 22 33 44 55 66 77 88 99 1010 1111 1212 1313 1414 1515 1616\n",
			want: []NetInterface{{Name: "eth0", RxBytes: 11, TxBytes: 99}},
		},
		{
			name:  "loopback only",
			input: netDevHeader + "    lo: 5 1 0 0 0 0 0 0 5 1 0 0 0 0 0 0\n",
			want:  nil,
		},
		{
			name: "loopback skipped between other interfaces",
			input: netDevHeader +
				"eth0: 1 0 0 0 0 0 0 0 2 0 0 0 0 0 0 0\n" +
				"lo: 9 0 0 0 0 0 0 0 9 0 0 0 0 0 0 0\n" +
				"eth1: 3 0 0 0 0 0 0 0 4 0 0 0 0 0 0 0\n",
			want: []NetInterface{
				{Name: "eth0", RxBytes: 1, TxBytes: 2},
				{Name: "eth1", RxBytes: 3, TxBytes: 4},
			},
		},
		{
			name:  "malformed loopback line is ignored",
			input: netDevHeader + "lo: oops\n",
			want:  nil,
		},
		{
			name:  "loopback with no counters is ignored",
			input: netDevHeader + "lo:\n",
			want:  nil,
		},
		{
			name:  "loopback with too few fields is ignored",
			input: netDevHeader + "lo: 1 2 3\n",
			want:  nil,
		},
		{
			name:  "loopback with non-numeric rx and tx is ignored",
			input: netDevHeader + "lo: abc 0 0 0 0 0 0 0 xyz 0 0 0 0 0 0 0\n",
			want:  nil,
		},
		{
			name:  "loopback with space before colon is ignored",
			input: netDevHeader + "  lo : oops\n",
			want:  nil,
		},
		{
			name: "malformed loopback line between valid interfaces",
			input: netDevHeader +
				"eth0: 1 0 0 0 0 0 0 0 2 0 0 0 0 0 0 0\n" +
				"lo: not a valid line\n" +
				"eth1: 3 0 0 0 0 0 0 0 4 0 0 0 0 0 0 0\n",
			want: []NetInterface{
				{Name: "eth0", RxBytes: 1, TxBytes: 2},
				{Name: "eth1", RxBytes: 3, TxBytes: 4},
			},
		},
		{
			name:  "names that only start with lo are kept",
			input: netDevHeader + "lo1: 1 0 0 0 0 0 0 0 2 0 0 0 0 0 0 0\n",
			want:  []NetInterface{{Name: "lo1", RxBytes: 1, TxBytes: 2}},
		},
		{
			name:  "no space after colon",
			input: netDevHeader + "eth0:1234567890123 1 0 0 0 0 0 0 98765432109 1 0 0 0 0 0 0\n",
			want:  []NetInterface{{Name: "eth0", RxBytes: 1234567890123, TxBytes: 98765432109}},
		},
		{
			name:  "space before colon and tabs",
			input: netDevHeader + "  eth0 :\t7\t0 0 0 0 0 0 0\t8 0 0 0 0 0 0 0\n",
			want:  []NetInterface{{Name: "eth0", RxBytes: 7, TxBytes: 8}},
		},
		{
			name:  "without trailing newline",
			input: netDevHeader + "eth0: 1 0 0 0 0 0 0 0 2 0 0 0 0 0 0 0",
			want:  []NetInterface{{Name: "eth0", RxBytes: 1, TxBytes: 2}},
		},
		{
			name:  "blank lines are ignored",
			input: netDevHeader + "\neth0: 1 0 0 0 0 0 0 0 2 0 0 0 0 0 0 0\n\n",
			want:  []NetInterface{{Name: "eth0", RxBytes: 1, TxBytes: 2}},
		},
		{
			name:  "header only",
			input: netDevHeader,
			want:  nil,
		},
		{
			name:  "header lines are skipped even if they look like data",
			input: "eth9: 1 0 0 0 0 0 0 0 2 0 0 0 0 0 0 0\neth8: 1 0 0 0 0 0 0 0 2 0 0 0 0 0 0 0\neth0: 5 0 0 0 0 0 0 0 6 0 0 0 0 0 0 0\n",
			want:  []NetInterface{{Name: "eth0", RxBytes: 5, TxBytes: 6}},
		},
		{name: "empty input", input: "", wantErr: true},
		{name: "only one header line", input: "Inter-|   Receive |  Transmit\n", wantErr: true},
		{name: "missing colon", input: netDevHeader + "eth0 1 0 0 0 0 0 0 0 2 0 0 0 0 0 0 0\n", wantErr: true},
		{name: "loopback line without colon is an error", input: netDevHeader + "lo 1 0 0 0 0 0 0 0 2 0 0 0 0 0 0 0\n", wantErr: true},
		{name: "empty interface name", input: netDevHeader + ": 1 0 0 0 0 0 0 0 2 0 0 0 0 0 0 0\n", wantErr: true},
		{name: "no counters", input: netDevHeader + "eth0:\n", wantErr: true},
		{name: "too few fields", input: netDevHeader + "eth0: 1 2 3\n", wantErr: true},
		{name: "eight fields only", input: netDevHeader + "eth0: 1 0 0 0 0 0 0 0\n", wantErr: true},
		{name: "non-numeric rx", input: netDevHeader + "eth0: abc 0 0 0 0 0 0 0 2 0 0 0 0 0 0 0\n", wantErr: true},
		{name: "non-numeric tx", input: netDevHeader + "eth0: 1 0 0 0 0 0 0 0 xyz 0 0 0 0 0 0 0\n", wantErr: true},
		{name: "negative rx", input: netDevHeader + "eth0: -1 0 0 0 0 0 0 0 2 0 0 0 0 0 0 0\n", wantErr: true},
		{name: "rx overflows uint64", input: netDevHeader + "eth0: 99999999999999999999 0 0 0 0 0 0 0 2 0 0 0 0 0 0 0\n", wantErr: true},
		{name: "malformed line for a lo-prefixed name is an error", input: netDevHeader + "lo1: oops\n", wantErr: true},
		{
			name: "malformed non-loopback line after a valid loopback line",
			input: netDevHeader +
				"lo: 1 0 0 0 0 0 0 0 2 0 0 0 0 0 0 0\n" +
				"eth0: broken\n",
			wantErr: true,
		},
		{
			name: "malformed non-loopback line after a malformed loopback line",
			input: netDevHeader +
				"lo: oops\n" +
				"eth0: broken\n",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseNetDev(strings.NewReader(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !sameInterfaces(got, tt.want) {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseNetDevMalformedLineAfterValidLine(t *testing.T) {
	input := netDevHeader +
		"eth0: 1 0 0 0 0 0 0 0 2 0 0 0 0 0 0 0\n" +
		"eth1: broken\n"

	got, err := ParseNetDev(strings.NewReader(input))
	if err == nil {
		t.Fatalf("expected error, got %+v", got)
	}
	if got != nil {
		t.Errorf("got %+v on error, want nil", got)
	}
	if !strings.Contains(err.Error(), "line 4") {
		t.Errorf("error %q does not mention the offending line number", err)
	}
}

func TestParseNetDevMalformedLoopbackDoesNotHideLaterError(t *testing.T) {
	input := netDevHeader +
		"lo: oops\n" +
		"eth1: broken\n"

	_, err := ParseNetDev(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for the malformed non-loopback line")
	}
	if !strings.Contains(err.Error(), "line 4") || !strings.Contains(err.Error(), "eth1") {
		t.Errorf("error %q should point at eth1 on line 4, not at the loopback line", err)
	}
}

func TestParseNetDevReaderError(t *testing.T) {
	sentinel := errors.New("boom")
	_, err := ParseNetDev(iotest.ErrReader(sentinel))
	if err == nil {
		t.Fatal("expected error from failing reader")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("error %v does not wrap the underlying reader error", err)
	}
}

func TestParseNetDevReaderErrorAfterValidData(t *testing.T) {
	sentinel := errors.New("read failed after data")
	r := io.MultiReader(strings.NewReader(netDevSyntheticSample), iotest.ErrReader(sentinel))

	got, err := ParseNetDev(r)
	if err == nil {
		t.Fatalf("expected reader error, got %+v and nil error", got)
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("error %v does not wrap the underlying reader error", err)
	}
	if got != nil {
		t.Errorf("got %+v on error, want nil", got)
	}
}
