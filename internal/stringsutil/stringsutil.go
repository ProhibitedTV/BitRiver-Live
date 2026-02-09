package stringsutil

import "strings"

// FirstNonEmpty returns the first non-empty trimmed value from candidates.
func FirstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
