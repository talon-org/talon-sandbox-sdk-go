package talonsandbox

import (
	"testing"
	"time"
)

func TestParseSize(t *testing.T) {
	cases := []struct {
		input   string
		want    int64
		wantErr bool
	}{
		{"4GiB", 4 * (1 << 30), false},
		{"4gib", 4 * (1 << 30), false},
		{"1GiB", 1 << 30, false},
		{"512MiB", 512 * (1 << 20), false},
		{"1KiB", 1 << 10, false},
		{"1TiB", 1 << 40, false},
		{"1GB", 1_000_000_000, false},
		{"1MB", 1_000_000, false},
		{"1KB", 1_000, false},
		{"1TB", 1_000_000_000_000, false},
		{"1B", 1, false},
		{"1024", 1024, false},
		{"0", 0, false},
		{"1.5GiB", int64(1.5 * float64(1<<30)), false},
		{"  4GiB  ", 4 * (1 << 30), false},
		{"10GiB", 10 * (1 << 30), false},
		{"256MiB", 256 * (1 << 20), false},
		{"2TiB", 2 * (1 << 40), false},
		{"100MB", 100_000_000, false},
		{"500KB", 500_000, false},
		{"", 0, true},
		{"-1GiB", 0, true},
		{"abc", 0, true},
		{"4XiB", 0, true},
		{"1.5.5GiB", 0, true},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got, err := ParseSize(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ParseSize(%q) = %d, want error", tc.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseSize(%q) error: %v", tc.input, err)
			}
			if got != tc.want {
				t.Fatalf("ParseSize(%q) = %d, want %d", tc.input, got, tc.want)
			}
		})
	}
}

func TestParseDuration(t *testing.T) {
	cases := []struct {
		input   string
		want    time.Duration
		wantErr bool
	}{
		{"30s", 30 * time.Second, false},
		{"5m", 5 * time.Minute, false},
		{"2h", 2 * time.Hour, false},
		{"1d", 24 * time.Hour, false},
		{"2d", 48 * time.Hour, false},
		{"1w", 7 * 24 * time.Hour, false},
		{"2w", 14 * 24 * time.Hour, false},
		{"30", 30 * time.Second, false},
		{"0", 0, false},
		{"100ms", 100 * time.Millisecond, false},
		{"1.5h", time.Duration(1.5 * float64(time.Hour)), false},
		{"1.5d", time.Duration(1.5 * float64(24*time.Hour)), false},
		{"  5m  ", 5 * time.Minute, false},
		{"6h", 6 * time.Hour, false},
		{"10m", 10 * time.Minute, false},
		{"3d", 72 * time.Hour, false},
		{"", 0, true},
		{"abc", 0, true},
		{"1y", 0, true},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got, err := ParseDuration(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ParseDuration(%q) = %v, want error", tc.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseDuration(%q) error: %v", tc.input, err)
			}
			if got != tc.want {
				t.Fatalf("ParseDuration(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}
