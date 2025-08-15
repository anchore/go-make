package file_test

import (
	"testing"

	"github.com/anchore/go-make/file"
	"github.com/anchore/go-make/require"
)

func Test_Sha256Hash(t *testing.T) {
	hash := file.Sha256Hash("testdata/some/.other/.config.json")
	require.Equal(t, "078d78c735ce63b3bb6c1dfac373de149d3ce0510e24fa43a497f5aef65ca715", hash)
}
