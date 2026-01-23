package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	defaultTOTPPeriodSeconds = 30
	defaultTOTPDigits        = 6
	defaultMFASecretBytes    = 20
	defaultRecoveryCodeBytes = 6
)

var (
	errMFACodeRequired = errors.New("mfa code required")
	errMFASecret       = errors.New("mfa secret required")
)

// GenerateMFASecret creates a new base32-encoded TOTP secret.
func GenerateMFASecret() (string, error) {
	secret := make([]byte, defaultMFASecretBytes)
	if _, err := rand.Read(secret); err != nil {
		return "", fmt.Errorf("generate mfa secret: %w", err)
	}
	return strings.TrimRight(base32.StdEncoding.EncodeToString(secret), "="), nil
}

// BuildOTPAuthURL builds the otpauth:// URL for an authenticator app.
func BuildOTPAuthURL(issuer, account, secret string) (string, error) {
	issuer = strings.TrimSpace(issuer)
	account = strings.TrimSpace(account)
	secret = strings.TrimSpace(secret)
	if issuer == "" || account == "" || secret == "" {
		return "", fmt.Errorf("issuer, account, and secret are required")
	}
	label := url.PathEscape(fmt.Sprintf("%s:%s", issuer, account))
	query := url.Values{}
	query.Set("secret", secret)
	query.Set("issuer", issuer)
	query.Set("algorithm", "SHA1")
	query.Set("digits", fmt.Sprintf("%d", defaultTOTPDigits))
	query.Set("period", fmt.Sprintf("%d", defaultTOTPPeriodSeconds))
	return fmt.Sprintf("otpauth://totp/%s?%s", label, query.Encode()), nil
}

// NormalizeMFACode normalizes user-entered MFA codes for comparison.
func NormalizeMFACode(code string) string {
	trimmed := strings.TrimSpace(code)
	if trimmed == "" {
		return ""
	}
	replacer := strings.NewReplacer(" ", "", "-", "")
	normalized := strings.ToUpper(replacer.Replace(trimmed))
	return normalized
}

// GenerateRecoveryCodes creates a list of formatted recovery codes.
func GenerateRecoveryCodes(count int) ([]string, error) {
	if count <= 0 {
		return nil, fmt.Errorf("recovery code count must be positive")
	}
	codes := make([]string, 0, count)
	for i := 0; i < count; i++ {
		bytes := make([]byte, defaultRecoveryCodeBytes)
		if _, err := rand.Read(bytes); err != nil {
			return nil, fmt.Errorf("generate recovery code: %w", err)
		}
		encoded := strings.TrimRight(base32.StdEncoding.EncodeToString(bytes), "=")
		encoded = strings.ToUpper(encoded)
		if len(encoded) > 10 {
			encoded = encoded[:10]
		}
		codes = append(codes, formatRecoveryCode(encoded))
	}
	return codes, nil
}

// HashRecoveryCode hashes a recovery code for storage.
func HashRecoveryCode(code string) (string, error) {
	normalized := NormalizeMFACode(code)
	if normalized == "" {
		return "", errMFACodeRequired
	}
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:]), nil
}

// MatchRecoveryCode compares a stored hash to a candidate code.
func MatchRecoveryCode(hash, candidate string) bool {
	normalized := NormalizeMFACode(candidate)
	if normalized == "" || hash == "" {
		return false
	}
	sum := sha256.Sum256([]byte(normalized))
	expected := hex.EncodeToString(sum[:])
	if len(expected) != len(hash) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expected), []byte(hash)) == 1
}

// VerifyTOTP verifies a user-provided TOTP code against the provided secret.
func VerifyTOTP(secret, code string, now time.Time) (bool, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return false, errMFASecret
	}
	normalized := NormalizeMFACode(code)
	if normalized == "" {
		return false, errMFACodeRequired
	}
	for _, offset := range []int64{-1, 0, 1} {
		if matchTOTP(secret, normalized, now.Add(time.Duration(offset)*time.Second*defaultTOTPPeriodSeconds)) {
			return true, nil
		}
	}
	return false, nil
}

func matchTOTP(secret, code string, now time.Time) bool {
	generated, err := totpCode(secret, now)
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(generated), []byte(code)) == 1
}

func totpCode(secret string, now time.Time) (string, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return "", errMFASecret
	}
	decoder := base32.StdEncoding.WithPadding(base32.NoPadding)
	key, err := decoder.DecodeString(strings.ToUpper(secret))
	if err != nil {
		return "", fmt.Errorf("decode secret: %w", err)
	}
	counter := uint64(now.Unix() / defaultTOTPPeriodSeconds)
	var payload [8]byte
	binary.BigEndian.PutUint64(payload[:], counter)
	mac := hmac.New(sha1.New, key)
	_, _ = mac.Write(payload[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	codeInt := (int(sum[offset])&0x7f)<<24 |
		(int(sum[offset+1])&0xff)<<16 |
		(int(sum[offset+2])&0xff)<<8 |
		(int(sum[offset+3]) & 0xff)
	mod := codeInt % int(pow10(defaultTOTPDigits))
	return fmt.Sprintf("%0*d", defaultTOTPDigits, mod), nil
}

func pow10(exp int) int {
	result := 1
	for i := 0; i < exp; i++ {
		result *= 10
	}
	return result
}

func formatRecoveryCode(raw string) string {
	raw = strings.ReplaceAll(raw, "-", "")
	if len(raw) <= 5 {
		return raw
	}
	return raw[:5] + "-" + raw[5:]
}
