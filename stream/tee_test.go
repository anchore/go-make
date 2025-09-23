package stream

import (
	"bytes"
	"io"
	"testing"

	"github.com/anchore/go-make/require"
)

func Test_TeeWriter(t *testing.T) {
	tests := []struct {
		name string
		w1   io.Writer
		w2   io.Writer
	}{
		{
			name: "two buffers",
			w1:   &bytes.Buffer{},
			w2:   &bytes.Buffer{},
		},
		{
			name: "no buffers",
		},
		{
			name: "buffer 1",
			w1:   &bytes.Buffer{},
		},
		{
			name: "buffer 2",
			w2:   &bytes.Buffer{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			teed := Tee()
			if tt.w1 != nil {
				teed.AddWriter(tt.w1)
			}
			if tt.w2 != nil {
				teed.AddWriter(tt.w2)
			}
			_, e := teed.Write([]byte("more"))
			require.NoError(t, e)

			_, e = teed.Write([]byte(" "))
			require.NoError(t, e)

			_, e = teed.Write([]byte("text"))
			require.NoError(t, e)

			if tt.w1 != nil {
				require.Equal(t, "more text", tt.w1.(*bytes.Buffer).String())
			}
			if tt.w2 != nil {
				require.Equal(t, "more text", tt.w2.(*bytes.Buffer).String())
			}
		})
	}
}
