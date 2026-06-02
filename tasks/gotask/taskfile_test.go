package gotask

import (
	"testing"

	"github.com/anchore/go-make/require"
)

func TestParseTaskListing(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want []taskInfo
	}{
		{
			name: "empty output yields no tasks",
			out:  "",
			want: nil,
		},
		{
			name: "names and descriptions are extracted",
			out: `{
				"tasks": [
					{"name": "build", "desc": "build the binary"},
					{"name": "internal", "desc": ""},
					{"name": "test", "desc": "run tests"}
				],
				"location": "/proj/Taskfile.yaml"
			}`,
			want: []taskInfo{
				{Name: "build", Desc: "build the binary"},
				{Name: "internal", Desc: ""},
				{Name: "test", Desc: "run tests"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, parseTaskListing(tt.out))
		})
	}
}

func TestMatchesAny(t *testing.T) {
	tests := []struct {
		name     string
		taskName string
		globs    []string
		want     bool
	}{
		{
			name:     "no globs matches everything",
			taskName: "build",
			globs:    nil,
			want:     true,
		},
		{
			name:     "exact name matches",
			taskName: "build",
			globs:    []string{"build"},
			want:     true,
		},
		{
			name:     "exact name does not match other task",
			taskName: "test",
			globs:    []string{"build"},
			want:     false,
		},
		{
			name:     "prefix glob matches namespaced task",
			taskName: "db:migrate",
			globs:    []string{"db:*"},
			want:     true,
		},
		{
			name:     "prefix glob does not match unrelated task",
			taskName: "build",
			globs:    []string{"db:*"},
			want:     false,
		},
		{
			name:     "matches when any glob matches",
			taskName: "test",
			globs:    []string{"build", "test", "db:*"},
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, matchesAny(tt.taskName, tt.globs))
		})
	}
}
