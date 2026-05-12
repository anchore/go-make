package gotest

import (
	"testing"

	"github.com/anchore/go-make/require"
)

func Test_scrubCoverLines(t *testing.T) {
	const modPath = "github.com/example/mod"

	// present holds the rel paths the fake fileExists should report as existing.
	tests := []struct {
		name        string
		present     []string
		lines       []string
		wantKept    []string
		wantDropped []string
	}{
		{
			name:    "preserves header, blanks, and existing in-module rows",
			present: []string{"foo/foo.go"},
			lines: []string{
				"mode: atomic",
				"github.com/example/mod/foo/foo.go:1.1,2.2 1 1",
				"",
			},
			wantKept: []string{
				"mode: atomic",
				"github.com/example/mod/foo/foo.go:1.1,2.2 1 1",
				"",
			},
		},
		{
			name:    "drops in-module rows whose file is missing",
			present: []string{"foo/foo.go"},
			lines: []string{
				"mode: atomic",
				"github.com/example/mod/foo/foo.go:1.1,2.2 1 1",
				"github.com/example/mod/bar/gone.go:1.1,2.2 1 0",
			},
			wantKept: []string{
				"mode: atomic",
				"github.com/example/mod/foo/foo.go:1.1,2.2 1 1",
			},
			wantDropped: []string{"github.com/example/mod/bar/gone.go"},
		},
		{
			name: "passes through rows outside the module",
			lines: []string{
				"mode: atomic",
				"github.com/other/mod/x.go:1.1,2.2 1 1",
			},
			wantKept: []string{
				"mode: atomic",
				"github.com/other/mod/x.go:1.1,2.2 1 1",
			},
		},
		{
			name: "keeps malformed rows with no colon",
			lines: []string{
				"mode: atomic",
				"this-row-has-no-colon",
			},
			wantKept: []string{
				"mode: atomic",
				"this-row-has-no-colon",
			},
		},
		{
			name: "drops every row for a missing file and preserves input order",
			lines: []string{
				"mode: atomic",
				"github.com/example/mod/bar/gone.go:1.1,2.2 1 0",
				"github.com/example/mod/bar/gone.go:3.1,4.2 1 0",
				"github.com/example/mod/bar/gone.go:5.1,6.2 1 0",
			},
			wantKept: []string{
				"mode: atomic",
			},
			// only one entry in dropped — memoized after the first miss.
			wantDropped: []string{"github.com/example/mod/bar/gone.go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			present := map[string]bool{}
			for _, p := range tt.present {
				present[p] = true
			}
			calls := map[string]int{}
			fileExists := func(rel string) bool {
				calls[rel]++
				return present[rel]
			}

			gotKept, gotDropped := scrubCoverLines(tt.lines, modPath, fileExists)

			require.Equal(t, tt.wantKept, gotKept)
			require.Equal(t, tt.wantDropped, gotDropped)

			// memoization: every checked file should be stat'd at most once.
			for rel, n := range calls {
				if n > 1 {
					t.Fatalf("fileExists(%q) called %d times; expected memoization", rel, n)
				}
			}
		})
	}
}
