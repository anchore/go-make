package gotest

import (
	"testing"

	"github.com/anchore/go-make/require"
)

func Test_parseCoveragePercent(t *testing.T) {
	tests := []struct {
		name      string
		report    string
		wantRaw   string
		wantPct   float64
		wantFound bool
	}{
		{
			name: "parses total coverage from go tool cover output",
			report: "github.com/foo/bar/baz.go:10:		Foo	100.0%\n" +
				"total:					(statements)	73.4%\n",
			wantRaw:   "73.4",
			wantPct:   73.4,
			wantFound: true,
		},
		{
			name:      "handles zero coverage",
			report:    "total:		(statements)	0.0%\n",
			wantRaw:   "0.0",
			wantPct:   0.0,
			wantFound: true,
		},
		{
			name:      "handles full coverage",
			report:    "total:		(statements)	100.0%\n",
			wantRaw:   "100.0",
			wantPct:   100.0,
			wantFound: true,
		},
		{
			name:      "preserves multi-decimal precision in raw output",
			report:    "total:		(statements)	79.95%\n",
			wantRaw:   "79.95",
			wantPct:   79.95,
			wantFound: true,
		},
		{
			name:      "returns false when no total line present",
			report:    "github.com/foo/bar/baz.go:10:	Foo	100.0%\n",
			wantFound: false,
		},
		{
			name:      "returns false on empty input",
			report:    "",
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, pct, found := parseCoveragePercent(tt.report)
			require.Equal(t, tt.wantFound, found)
			if !found {
				return
			}
			require.Equal(t, tt.wantRaw, raw)
			require.Equal(t, tt.wantPct, pct)
		})
	}
}

func Test_enforceCoverageThreshold(t *testing.T) {
	tests := []struct {
		name      string
		rawPct    string
		coverage  float64
		found     bool
		threshold float64
		wantErr   string
	}{
		{
			name:      "no threshold means no error even if unparseable",
			found:     false,
			threshold: 0,
		},
		{
			name:      "negative threshold disables the check",
			rawPct:    "10.0",
			coverage:  10,
			found:     true,
			threshold: -1,
		},
		{
			name:      "coverage equal to threshold passes",
			rawPct:    "80.0",
			coverage:  80.0,
			found:     true,
			threshold: 80.0,
		},
		{
			name:      "coverage above threshold passes",
			rawPct:    "85.5",
			coverage:  85.5,
			found:     true,
			threshold: 80.0,
		},
		{
			name:      "coverage below threshold fails",
			rawPct:    "79.9",
			coverage:  79.9,
			found:     true,
			threshold: 80.0,
			wantErr:   "coverage 79.9% is below threshold 80%",
		},
		{
			name:      "uses raw string to avoid rounding ambiguity",
			rawPct:    "79.96",
			coverage:  79.96,
			found:     true,
			threshold: 80.0,
			wantErr:   "coverage 79.96% is below threshold 80%",
		},
		{
			name:      "fractional threshold renders without trailing zero",
			rawPct:    "80.0",
			coverage:  80.0,
			found:     true,
			threshold: 80.5,
			wantErr:   "coverage 80.0% is below threshold 80.5%",
		},
		{
			name:      "threshold set but coverage unparseable fails",
			found:     false,
			threshold: 80.0,
			wantErr:   "coverage threshold 80% set but coverage percentage could not be determined",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := enforceCoverageThreshold(tt.rawPct, tt.coverage, tt.found, tt.threshold)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.Equal(t, tt.wantErr, err.Error())
		})
	}
}
