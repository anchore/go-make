package shell

import (
	"strings"
)

// Split splits a string at spaces, taking into account shell quotes and {{template directives}}
func Split(s string) []string {
	var out []string
	start := 0
	quote := rune(0)
	for i, ch := range s {
		switch ch {
		case '{':
			quote = ch
		case '}':
			quote = 0
		case '\'', '"', '`':
			if quote == ch {
				out = append(out, s[start:i])
				start = i + 1
				quote = 0
				continue
			}
			if quote > 0 {
				continue
			}
			quote = ch
			if i > start {
				out = append(out, s[start:i])
			}
			start = i + 1
		case ' ', '\t', '\r', '\n':
			if quote > 0 {
				break
			}
			v := strings.TrimSpace(s[start:i])
			if len(v) > 0 {
				out = append(out, v)
			}
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

// Flatten splits comma separated lists into a single list
func Flatten(commaSeparatedStrings ...string) []string {
	return DelimiterFlatten(",", commaSeparatedStrings...)
}

// DelimiterFlatten splits lists of strings on a delimiter into a single flattened list
func DelimiterFlatten(delimiter string, delimiterSeparatedStrings ...string) []string {
	var out []string
	for _, delimiterSeparatedString := range delimiterSeparatedStrings {
		for _, s := range strings.Split(delimiterSeparatedString, delimiter) {
			out = append(out, strings.TrimSpace(s))
		}
	}
	return out
}
