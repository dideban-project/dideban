package collector

import (
	"errors"
	"strings"
	"testing"
	"testing/iotest"
)

func TestParseLoadavg(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    LoadAvg
		wantErr bool
	}{
		{
			name:  "valid real-world line",
			input: "0.20 0.18 0.12 1/80 11206\n",
			want:  LoadAvg{Load1: 0.20, Load5: 0.18, Load15: 0.12},
		},
		{
			name:  "valid without trailing newline",
			input: "1.50 2.25 3.75 4/512 9999",
			want:  LoadAvg{Load1: 1.50, Load5: 2.25, Load15: 3.75},
		},
		{
			name:  "extra whitespace",
			input: "  0.01\t0.02   0.03  1/2 3  \n",
			want:  LoadAvg{Load1: 0.01, Load5: 0.02, Load15: 0.03},
		},
		{
			name:  "only three fields",
			input: "0.10 0.20 0.30\n",
			want:  LoadAvg{Load1: 0.10, Load5: 0.20, Load15: 0.30},
		},
		{
			name:  "high load",
			input: "128.75 96.50 64.25 200/4096 123456\n",
			want:  LoadAvg{Load1: 128.75, Load5: 96.50, Load15: 64.25},
		},
		{
			name:  "zero load",
			input: "0.00 0.00 0.00 1/100 1\n",
			want:  LoadAvg{},
		},
		{name: "empty input", input: "", wantErr: true},
		{name: "whitespace only", input: "  \n", wantErr: true},
		{name: "too few fields", input: "0.20 0.18\n", wantErr: true},
		{name: "non-numeric load1", input: "abc 0.18 0.12 1/80 1\n", wantErr: true},
		{name: "non-numeric load5", input: "0.20 x 0.12 1/80 1\n", wantErr: true},
		{name: "non-numeric load15", input: "0.20 0.18 -- 1/80 1\n", wantErr: true},
		{name: "negative value", input: "-0.20 0.18 0.12 1/80 1\n", wantErr: true},
		{name: "NaN value", input: "NaN 0.18 0.12 1/80 1\n", wantErr: true},
		{name: "infinite value", input: "0.20 Inf 0.12 1/80 1\n", wantErr: true},
		{name: "comma decimal separator", input: "0,20 0,18 0,12 1/80 1\n", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseLoadavg(strings.NewReader(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %+v", got)
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

func TestParseLoadavgReaderError(t *testing.T) {
	sentinel := errors.New("boom")
	_, err := ParseLoadavg(iotest.ErrReader(sentinel))
	if err == nil {
		t.Fatal("expected error from failing reader")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("error %v does not wrap the underlying reader error", err)
	}
}
