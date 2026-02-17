package stream_test

import (
	"regexp"
	"testing"

	"github.com/anchore/go-make/require"
	"github.com/anchore/go-make/stream"
)

func Test_regexScanner(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		regex *regexp.Regexp
		size  int
		match []map[string]string
	}{
		{
			name:  "input chunks smaller than buf",
			input: []string{"a", "b", "c"},
			regex: regexp.MustCompile(regexp.QuoteMeta("ab")),
			match: []map[string]string{
				{"": "ab"},
			},
		},
		{
			name:  "input chunks larger than buf",
			size:  4,
			input: []string{"abcdefg", "abcdefg"},
			regex: regexp.MustCompile(regexp.QuoteMeta("b")),
			match: []map[string]string{
				{"": "b"},
				{"": "b"},
			},
		},
		{
			name:  "end input chunks larger than buf",
			size:  4,
			input: []string{"abcdefg", "abcdefg"},
			regex: regexp.MustCompile(regexp.QuoteMeta("fg")),
			match: []map[string]string{
				{"": "fg"},
				{"": "fg"},
			},
		},
		{
			name:  "search longer than inputs",
			size:  4,
			input: []string{"ab", "cd", "ef", "g", "ab", "cd", "ef", "g"},
			regex: regexp.MustCompile(regexp.QuoteMeta("abc")),
			match: []map[string]string{
				{"": "abc"},
				{"": "abc"},
			},
		},
		{
			name:  "sub expressions",
			size:  4,
			input: []string{"abcdefg", "abcdefg"},
			regex: regexp.MustCompile("a(?P<thename>b)"),
			match: []map[string]string{
				{"": "ab", "thename": "b"},
				{"": "ab", "thename": "b"},
			},
		},
		{
			name:  "large input",
			size:  16,
			input: chunk(4, `init process ; init process ; ready for start up.`),
			regex: regexp.MustCompile(regexp.QuoteMeta("ready for start")),
			match: []map[string]string{
				{"": "ready for start"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var opts []stream.Option
			if tt.size > 0 {
				opts = append(opts, stream.Size(tt.size))
			}
			r, result := stream.NewRegexpScanner(tt.regex, opts...)
			f := func() {
				for _, s := range tt.input {
					_, err := r.Write([]byte(s))
					require.NoError(t, err)
				}
			}
			go func() {
				f()
				close(result)
			}()

			var matches []map[string]string
			for m := range result {
				matches = append(matches, m)
			}

			require.Equal(t, tt.match, matches)
		})
	}
}

func chunk(size int, input string) []string {
	var out []string
	for i := 0; i < len(input); i += size {
		if i+size > len(input) {
			size = len(input) - i
		}
		out = append(out, input[i:i+size])
	}
	return out
}
