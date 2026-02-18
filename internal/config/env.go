package config

import (
	"os"
	"strings"
)

// Environment provides read access to process configuration values.
type Environment struct {
	values map[string]string
}

// LoadEnvironment snapshots the current process environment.
func LoadEnvironment() Environment {
	values := make(map[string]string)
	for _, entry := range os.Environ() {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		values[key] = value
	}
	return Environment{values: values}
}

// Get returns the environment variable value or empty string.
func (e Environment) Get(key string) string {
	if e.values == nil {
		return ""
	}
	return e.values[key]
}

// Lookup returns an environment variable value and whether it exists.
func (e Environment) Lookup(key string) (string, bool) {
	if e.values == nil {
		return "", false
	}
	v, ok := e.values[key]
	return v, ok
}
