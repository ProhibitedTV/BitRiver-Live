package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
)

var errSessionTokenRequired = errors.New("session token required")

func hashSessionToken(token string) (string, error) {
	if token == "" {
		return "", errSessionTokenRequired
	}
	digest := sha256.Sum256([]byte(token))
	return hex.EncodeToString(digest[:]), nil
}

func generateHashedSessionToken(length int) (string, string, error) {
	token, err := generateToken(length)
	if err != nil {
		return "", "", err
	}
	hashed, err := hashSessionToken(token)
	if err != nil {
		return "", "", err
	}
	return token, hashed, nil
}
