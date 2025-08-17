package file_test

import (
	"testing"

	"github.com/anchore/go-make/file"
	"github.com/anchore/go-make/require"
)

func Test_Sha256Hash(t *testing.T) {
	hash := file.Sha256Hash("testdata/some/.other/.config.json")
	require.Equal(t, "c9a17ab03f15b9a0e346cba66ea35c621d43f05911b378babc8c86411dd9669c", hash)
}
