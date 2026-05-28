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
