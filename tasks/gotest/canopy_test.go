package gotest

import (
	"testing"

	"github.com/anchore/go-make/require"
)

func Test_buildCanopyArgs(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want []string
	}{
		{
			name: "defaults",
			cfg:  Config{Name: "unit", IncludeGlob: "./...", Coverage: true},
			want: []string{"test", "./...", "--cover", "--covermode", "atomic", "--coverpkg", "./...", "--tags", "coverage"},
		},
		{
			name: "no coverage drops the cover flags and the coverage build tag",
			cfg:  Config{Name: "unit", IncludeGlob: "./..."},
			want: []string{"test", "./..."},
		},
		{
			name: "threshold becomes --covermin",
			cfg:  Config{IncludeGlob: "./...", Coverage: true, CoverageThreshold: 80},
			want: []string{"test", "./...", "--cover", "--covermode", "atomic", "--coverpkg", "./...", "--covermin", "80", "--tags", "coverage"},
		},
		{
			name: "fractional threshold keeps its precision",
			cfg:  Config{IncludeGlob: "./...", Coverage: true, CoverageThreshold: 79.95},
			want: []string{"test", "./...", "--cover", "--covermode", "atomic", "--coverpkg", "./...", "--covermin", "79.95", "--tags", "coverage"},
		},
		{
			name: "run filter, tags, and race use canopy's long-form flags",
			cfg:  Config{IncludeGlob: "./git/...", RunFilter: "TestFoo", Tags: []string{"integration"}, Race: true},
			want: []string{"test", "--run", "TestFoo", "./git/...", "--tags", "integration", "--race"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, buildCanopyArgs(&tt.cfg))
		})
	}
}
