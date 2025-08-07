package shell_test

import (
	"testing"

	. "github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/require"
	"github.com/anchore/go-make/shell"
)

func Test_ShellSplit(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{
			input:    "one",
			expected: List("one"),
		},
		{
			input:    "t wo",
			expected: List("t", "wo"),
		},
		{
			input:    "th 'r ee'",
			expected: List("th", "r ee"),
		},
		{
			input:    "th 'r ee' four",
			expected: List("th", "r ee", "four"),
		},
		{
			input:    " pre",
			expected: List("pre"),
		},
		{
			input:    "post ",
			expected: List("post"),
		},
		{
			input:    " ' one ' ",
			expected: List(" one "),
		},
		{
			input:    `{{some template 'stuff' }} should 'be ' "ver ba tim" `,
			expected: List(`{{some template 'stuff' }}`, `should`, `be `, `ver ba tim`),
		},
		{
			input:    ` a 'very real"istic ' "te'st" with    lo\ts of  	'sp/\ces' ' her"e" ' `,
			expected: List(`a`, `very real"istic `, `te'st`, `with`, `lo\ts`, `of`, `sp/\ces`, ` her"e" `),
		},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := shell.Split(tt.input)
			require.EqualElements(t, tt.expected, got)
		})
	}
}
