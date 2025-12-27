package envutil

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// FromEnviron converts a slice of KEY=VALUE entries into a map.
func FromEnviron(environ []string) map[string]string {
	values := make(map[string]string, len(environ))

	for _, entry := range environ {
		if idx := strings.Index(entry, "="); idx != -1 {
			key := entry[:idx]
			val := entry[idx+1:]
			values[key] = val
		}
	}

	return values
}

// LoadFile parses a .env-style file and merges the values into base.
// Blank lines and comments (lines starting with #) are ignored. The
// provided base map is copied before modification; if nil, an empty map is used.
// If the file does not exist, the base map is returned unchanged.
func LoadFile(path string, base map[string]string) (map[string]string, error) {
	values := make(map[string]string, len(base))
	for k, v := range base {
		values[k] = v
	}

	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return values, nil
		}
		return nil, fmt.Errorf("open env file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		idx := strings.Index(line, "=")
		if idx == -1 {
			continue
		}

		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])

		if len(val) >= 2 && strings.HasPrefix(val, "\"") && strings.HasSuffix(val, "\"") {
			if unquoted, err := strconv.Unquote(val); err == nil {
				val = unquoted
			}
		}

		values[key] = val
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan env file: %w", err)
	}

	return values, nil
}

// FirstExistingPath returns the first path that exists from the given candidates.
// The order of candidates is preserved. If no candidates exist, os.ErrNotExist is returned.
func FirstExistingPath(candidates []string) (string, error) {
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("stat candidate %s: %w", candidate, err)
		}
	}

	return "", os.ErrNotExist
}
