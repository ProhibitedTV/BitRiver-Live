package storage

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
)

// generateID performs generate id and propagates validation or dependency failures to the caller.
func generateID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// generateStreamKey performs generate stream key and propagates validation or dependency failures to the caller.
func generateStreamKey() (string, error) {
	bytes := make([]byte, 24)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate stream key: %w", err)
	}
	return strings.ToUpper(hex.EncodeToString(bytes)), nil
}
