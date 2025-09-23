package run

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anchore/go-make/config"
	. "github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/log"
	"github.com/anchore/go-make/require"
)

func Test_Command(t *testing.T) {
	buf1 := bytes.Buffer{}
	buf2 := bytes.Buffer{}

	tmpDir := t.TempDir()
	testapp := filepath.Join(tmpDir, "testapp")
	if config.Windows {
		testapp += ".exe"
	}
	_, err := Command("go", Args("build", "-C", filepath.Join("testdata", "testapp"), "-o", testapp, "."))
	require.NoError(t, err)

	tests := []struct {
		name     string
		args     []Option
		validate func(t *testing.T, commandLog, result string)
		wantErr  require.ValidationError
	}{
		{
			name: "no buffering stdout returned",
			args: List(Args("stdout", "some-value")),
			validate: func(t *testing.T, commandLog, result string) {
				require.Contains(t, result, "some-value")
			},
		},
		{
			name: "buffered stdout does not return",
			args: List(Args("stdout", "some-value"), Stdout(&buf1)),
			validate: func(t *testing.T, commandLog, result string) {
				require.Equal(t, "", result)
				require.Contains(t, buf1.String(), "some-value")
			},
		},
		{
			name: "quiet does not prevent buf stdout",
			args: List(Args("stdout", "some-value"), Quiet(), Stdout(&buf1)),
			validate: func(t *testing.T, commandLog, result string) {
				require.Equal(t, "", result)
				require.Contains(t, buf1.String(), "some-value")
			},
		},
		{
			name:     "quiet does not prevent stderr on error",
			args:     List(Args("stdout", "some-stdout-value", "stderr", "some-stderr-value", "exit-code", "2"), Quiet()),
			validate: func(t *testing.T, commandLog, result string) {},
			wantErr: func(t *testing.T, err error) {
				require.Contains(t, err.Error(), "2")
				require.Contains(t, err.Error(), "some-stdout-value")
				require.Contains(t, err.Error(), "some-stderr-value")
			},
		},
		{
			name: "stdin",
			args: List(Args("stdin"), Quiet(), Stdin(strings.NewReader("some-stdin-value"))),
			validate: func(t *testing.T, commandLog, result string) {
				require.Equal(t, "", result)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			commandLog := ""
			require.SetAndRestore(t, &log.Info, func(format string, args ...any) {
				commandLog = fmt.Sprintf(format, args...)
			})
			result, err := Command(testapp, tt.args...)
			tt.wantErr.Validate(t, err)
			tt.validate(t, commandLog, result)
			buf1.Reset()
			buf2.Reset()
		})
	}
}
