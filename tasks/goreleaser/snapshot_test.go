package goreleaser

import (
	"testing"

	"github.com/anchore/go-make/require"
)

func Test_snapshotArgs(t *testing.T) {
	tests := []struct {
		name         string
		config       string
		singleTarget bool
		want         string
	}{
		{
			name:         "full snapshot uses release",
			config:       "/tmp/x/.goreleaser.yaml",
			singleTarget: false,
			want:         "goreleaser release --clean --snapshot --skip=publish --skip=sign --config=/tmp/x/.goreleaser.yaml",
		},
		{
			// --single-target is a build-only flag; `goreleaser release` rejects it.
			name:         "single-target uses build",
			config:       "/tmp/x/.goreleaser.yaml",
			singleTarget: true,
			want:         "goreleaser build --clean --snapshot --single-target --config=/tmp/x/.goreleaser.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, snapshotArgs(tt.config, tt.singleTarget))
		})
	}
}
