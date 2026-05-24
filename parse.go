package talonsandbox

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ParseSize converts a human-readable size string to bytes.
//
// Accepted suffixes (case-insensitive):
//
//	B, KB, MB, GB, TB — powers of 1000
//	KiB, MiB, GiB, TiB — powers of 1024
//
// A bare integer is treated as bytes. Floats are supported ("1.5GiB").
func ParseSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("ParseSize: empty string")
	}

	// Fast path: bare integer
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		if n < 0 {
			return 0, fmt.Errorf("ParseSize: negative value %q", s)
		}
		return n, nil
	}

	type unit struct {
		suffix string
		mult   int64
	}
	units := []unit{
		{"TiB", 1 << 40},
		{"GiB", 1 << 30},
		{"MiB", 1 << 20},
		{"KiB", 1 << 10},
		{"TB", 1_000_000_000_000},
		{"GB", 1_000_000_000},
		{"MB", 1_000_000},
		{"KB", 1_000},
		{"B", 1},
	}

	upper := strings.ToUpper(s)
	for _, u := range units {
		if strings.HasSuffix(upper, strings.ToUpper(u.suffix)) {
			numStr := strings.TrimSpace(s[:len(s)-len(u.suffix)])
			f, err := strconv.ParseFloat(numStr, 64)
			if err != nil {
				return 0, fmt.Errorf("ParseSize: invalid number in %q", s)
			}
			if f < 0 {
				return 0, fmt.Errorf("ParseSize: negative value %q", s)
			}
			return int64(f * float64(u.mult)), nil
		}
	}
	return 0, fmt.Errorf("ParseSize: unrecognised format %q (expected e.g. \"4GiB\", \"512MB\")", s)
}

// ParseDuration converts a duration string to time.Duration.
//
// Supports all stdlib time.ParseDuration formats plus:
//   - "d" suffix for days (1d = 24h)
//   - "w" suffix for weeks (1w = 168h)
//   - bare integer treated as seconds ("30" → 30s)
func ParseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("ParseDuration: empty string")
	}

	// Bare integer → seconds
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return time.Duration(n) * time.Second, nil
	}

	lower := strings.ToLower(s)

	// Week suffix
	if strings.HasSuffix(lower, "w") {
		numStr := s[:len(s)-1]
		f, err := strconv.ParseFloat(numStr, 64)
		if err != nil {
			return 0, fmt.Errorf("ParseDuration: invalid number in %q", s)
		}
		return time.Duration(f * float64(7*24*time.Hour)), nil
	}

	// Day suffix — "d" is not a stdlib suffix, so safe to check for it
	// before delegating to stdlib.
	if strings.HasSuffix(lower, "d") {
		numStr := s[:len(s)-1]
		f, err := strconv.ParseFloat(numStr, 64)
		if err != nil {
			return 0, fmt.Errorf("ParseDuration: invalid number in %q", s)
		}
		return time.Duration(f * float64(24*time.Hour)), nil
	}

	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("ParseDuration: %w", err)
	}
	return d, nil
}
