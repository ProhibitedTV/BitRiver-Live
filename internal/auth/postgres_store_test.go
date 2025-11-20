package auth

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/puddle/v2"
)

func TestHashSessionToken(t *testing.T) {
	token := "token-to-hash"

	hashed, err := hashSessionToken(token)
	if err != nil {
		t.Fatalf("hashSessionToken: %v", err)
	}
	if hashed == token {
		t.Fatalf("expected hashed token to differ from raw value")
	}

	repeat, err := hashSessionToken(token)
	if err != nil {
		t.Fatalf("hashSessionToken repeat: %v", err)
	}
	if hashed != repeat {
		t.Fatalf("expected hashing to be deterministic")
	}
}

func TestHashSessionTokenEmpty(t *testing.T) {
	if _, err := hashSessionToken(""); !errors.Is(err, errSessionTokenRequired) {
		t.Fatalf("expected empty token error, got %v", err)
	}
}

func TestGenerateHashedSessionToken(t *testing.T) {
	token, hashed, err := generateHashedSessionToken(8)
	if err != nil {
		t.Fatalf("generateHashedSessionToken: %v", err)
	}
	if token == "" || hashed == "" {
		t.Fatalf("expected non-empty token and hash")
	}

	derived, err := hashSessionToken(token)
	if err != nil {
		t.Fatalf("hashSessionToken: %v", err)
	}
	if derived != hashed {
		t.Fatalf("expected hash to match derived value")
	}
}

func TestIsNoRowsTrueForErrNoRows(t *testing.T) {
	if !isNoRows(pgx.ErrNoRows) {
		t.Fatalf("expected pgx.ErrNoRows to be treated as no rows")
	}
}

func TestIsNoRowsFalseForClosedPool(t *testing.T) {
	if isNoRows(puddle.ErrClosedPool) {
		t.Fatalf("expected closed pool error to not be treated as no rows")
	}
}

func TestIsNoRowsFalseForOtherError(t *testing.T) {
	if isNoRows(errors.New("boom")) {
		t.Fatalf("expected arbitrary error to not be treated as no rows")
	}
}
