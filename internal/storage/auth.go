package storage

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"bitriver-live/internal/models"
	"golang.org/x/crypto/pbkdf2"
)

// AuthenticateUser verifies credentials and returns the matching user on success.
func (s *Storage) AuthenticateUser(email, password string) (models.User, error) {
	if password == "" {
		return models.User{}, errors.New("password is required")
	}
	user, ok := s.FindUserByEmail(email)
	if !ok {
		return models.User{}, ErrInvalidCredentials
	}
	if user.PasswordHash == "" {
		return models.User{}, ErrPasswordLoginUnsupported
	}
	if err := verifyPassword(user.PasswordHash, password); err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			return models.User{}, ErrInvalidCredentials
		}
		return models.User{}, err
	}
	return user, nil
}

// SetUserPassword replaces the stored password hash for the provided user.
func (s *Storage) SetUserPassword(id, password string) (models.User, error) {
	if len(password) < 8 {
		return models.User{}, errors.New("password must be at least 8 characters")
	}

	hashed, err := hashPassword(password)
	if err != nil {
		return models.User{}, fmt.Errorf("hash password: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	updatedData := cloneDataset(s.data)

	user, ok := updatedData.Users[id]
	if !ok {
		return models.User{}, fmt.Errorf("user %s not found", id)
	}

	user.PasswordHash = hashed
	updatedData.Users[id] = user

	if err := s.persistDataset(updatedData); err != nil {
		return models.User{}, err
	}

	s.data = updatedData

	return user, nil
}

func hashPassword(password string) (string, error) {
	salt := make([]byte, passwordHashSaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	derived := pbkdf2.Key([]byte(password), salt, passwordHashIterations, passwordHashKeyLength, sha256.New)
	encodedSalt := base64.RawStdEncoding.EncodeToString(salt)
	encodedKey := base64.RawStdEncoding.EncodeToString(derived)
	return fmt.Sprintf("pbkdf2$sha256$%d$%s$%s", passwordHashIterations, encodedSalt, encodedKey), nil
}

func verifyPassword(encodedHash, candidate string) error {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 5 {
		return fmt.Errorf("verify password: invalid hash format")
	}
	if parts[0] != "pbkdf2" || parts[1] != "sha256" {
		return fmt.Errorf("verify password: unsupported hash identifier")
	}
	iterations, err := strconv.Atoi(parts[2])
	if err != nil || iterations <= 0 {
		return fmt.Errorf("verify password: invalid iteration count")
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return fmt.Errorf("verify password: decode salt: %w", err)
	}
	storedKey, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return fmt.Errorf("verify password: decode hash: %w", err)
	}
	derived := pbkdf2.Key([]byte(candidate), salt, iterations, len(storedKey), sha256.New)
	if len(derived) != len(storedKey) || subtle.ConstantTimeCompare(derived, storedKey) != 1 {
		return ErrInvalidCredentials
	}
	return nil
}
