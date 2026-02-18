package config

import (
	"os"
	"strings"
)

// Environment provides read access to process configuration values.
type Environment struct {
	values map[string]string
}

// NewEnvironmentFromMap builds an immutable environment snapshot from the provided map.
func NewEnvironmentFromMap(values map[string]string) Environment {
	cloned := make(map[string]string, len(values))
	for k, v := range values {
		cloned[k] = v
	}
	return Environment{values: cloned}
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
