package collector

import (
	"errors"
	"strings"
	"testing"
	"testing/iotest"
)

func TestParseUptime(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    float64
		wantErr bool
	}{
		{name: "issue example", input: "350735.47 234388.90", want: 350735.47},
		{name: "with trailing newline", input: "350735.47 234388.90\n", want: 350735.47},
		{name: "extra whitespace", input: "  12.50\t3.25  \n", want: 12.50},
		{name: "second number is ignored", input: "1.50 999999.99\n", want: 1.50},
		{name: "first number only", input: "42.00\n", want: 42.00},
		{name: "zero uptime", input: "0.00 0.00\n", want: 0},
		{name: "empty input", input: "", wantErr: true},
		{name: "whitespace only", input: "  \n", wantErr: true},
		{name: "non-numeric first field", input: "abc 234388.90\n", wantErr: true},
		{name: "non-numeric first but numeric second", input: "x 12.5\n", wantErr: true},
		{name: "negative value", input: "-1.00 234388.90\n", wantErr: true},
		{name: "NaN value", input: "NaN 234388.90\n", wantErr: true},
		{name: "infinite value", input: "Inf 234388.90\n", wantErr: true},
		{name: "comma decimal separator", input: "350735,47 234388,90\n", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseUptime(strings.NewReader(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseUptimeReaderError(t *testing.T) {
	sentinel := errors.New("boom")
	_, err := ParseUptime(iotest.ErrReader(sentinel))
	if err == nil {
		t.Fatal("expected error from failing reader")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("error %v does not wrap the underlying reader error", err)
	}
}

// errAfterDataReader returns data first and then fails with err (never io.EOF).
// If errWithLastData is true, the error is returned together with the final
// bytes in the same Read call instead of in a separate call.
type errAfterDataReader struct {
	data            string
	err             error
	errWithLastData bool
}

func (r *errAfterDataReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, r.err
	}
	n := copy(p, r.data)
	r.data = r.data[n:]
	if len(r.data) == 0 && r.errWithLastData {
		return n, r.err
	}
	return n, nil
}

func TestParseUptimeReaderErrorAfterValidLine(t *testing.T) {
	sentinel := errors.New("read failed after data")

	tests := []struct {
		name   string
		reader *errAfterDataReader
	}{
		{
			name:   "valid line with newline, error on next read",
			reader: &errAfterDataReader{data: "350735.47 234388.90\n", err: sentinel},
		},
		{
			name:   "valid line without newline, error on next read",
			reader: &errAfterDataReader{data: "350735.47 234388.90", err: sentinel},
		},
		{
			name:   "valid line, error returned with the last data",
			reader: &errAfterDataReader{data: "350735.47 234388.90\n", err: sentinel, errWithLastData: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseUptime(tt.reader)
			if err == nil {
				t.Fatalf("expected reader error, got value %v and nil error", got)
			}
			if !errors.Is(err, sentinel) {
				t.Errorf("error %v does not wrap the underlying reader error", err)
			}
			if got != 0 {
				t.Errorf("got %v on error, want 0", got)
			}
		})
	}
}
