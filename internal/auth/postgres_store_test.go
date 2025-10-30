package auth

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/puddle/v2"
)

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
