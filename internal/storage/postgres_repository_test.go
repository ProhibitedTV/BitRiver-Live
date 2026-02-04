package storage

import (
	"errors"
	"strings"
	"testing"
)

func TestWrapPostgresUnavailableIncludesCause(t *testing.T) {
	root := errors.New("dial tcp: connection refused")
	err := wrapPostgresUnavailable(root)
	if !errors.Is(err, ErrPostgresUnavailable) {
		t.Fatalf("expected ErrPostgresUnavailable; got %v", err)
	}
	if !errors.Is(err, root) {
		t.Fatalf("expected root cause in error chain; got %v", err)
	}
	if !strings.Contains(err.Error(), root.Error()) {
		t.Fatalf("expected error to include root cause string; got %q", err.Error())
	}
}

func TestNewPostgresRepositoryStubbedIncludesCause(t *testing.T) {
	repo, err := NewPostgresRepository("postgres://example")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if repo != nil {
		t.Fatalf("expected nil repo, got %T", repo)
	}
	if !errors.Is(err, ErrPostgresUnavailable) {
		t.Fatalf("expected ErrPostgresUnavailable; got %v", err)
	}
	if !strings.Contains(err.Error(), "pgx driver stubbed") {
		t.Fatalf("expected stub cause string; got %q", err.Error())
	}
}
