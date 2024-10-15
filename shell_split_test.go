package make_test

import (
	"testing"

	. "github.com/anchore/go-make"
)

func Test_ShellSplit(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{
			input:    "one",
			expected: All("one"),
		},
		{
			input:    "t wo",
			expected: All("t", "wo"),
		},
		{
			input:    "th 'r ee'",
			expected: All("th", "r ee"),
		},
		{
			input:    "th 'r ee' four",
			expected: All("th", "r ee", "four"),
		},
		{
			input:    " pre",
			expected: All("pre"),
		},
		{
			input:    "post ",
			expected: All("post"),
		},
		{
			input:    " ' one ' ",
			expected: All(" one "),
		},
		{
			input:    `{{some template 'stuff' }} should 'be ' "ver ba tim" `,
			expected: All(`{{some template 'stuff' }}`, `should`, `be `, `ver ba tim`),
		},
		{
			input:    ` a 'very real"istic ' "te'st" with    lo\ts of  	'sp/\ces' ' her"e" ' `,
			expected: All(`a`, `very real"istic `, `te'st`, `with`, `lo\ts`, `of`, `sp/\ces`, ` her"e" `),
		},
	}
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			got := ShellSplit(test.input)
			requireEqualElements(t, test.expected, got)
		})
	}
}
