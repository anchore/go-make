package log

import "encoding/json"

// FormatJSON pretty-prints the provided JSON string, returning the original string if an error occurs
func FormatJSON(contents string) string {
	m := map[string]any{}
	err := json.Unmarshal([]byte(contents), &m)
	if err != nil {
		Trace("unable to format JSON: %v", err)
		return contents
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		Trace("unable to format JSON: %v", err)
		return contents
	}
	return string(b)
}
