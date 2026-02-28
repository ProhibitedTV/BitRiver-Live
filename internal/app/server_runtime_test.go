package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"bitriver-live/internal/auth"
	"bitriver-live/internal/chat"
	"bitriver-live/internal/observability/tracing"
	"bitriver-live/internal/storage"
)

type testRepo struct {
	storage.Repository
	closeCalls int
}

func (r *testRepo) Close(context.Context) error {
	r.closeCalls++
	return nil
}

type testSessionStore struct{ closeCalls int }

func (s *testSessionStore) Save(string, string, time.Time, time.Time) error { return nil }
func (s *testSessionStore) Get(string) (auth.SessionRecord, bool, error) {
	return auth.SessionRecord{}, false, nil
}
func (s *testSessionStore) Delete(string) error          { return nil }
func (s *testSessionStore) PurgeExpired(time.Time) error { return nil }
func (s *testSessionStore) Close(context.Context) error {
	s.closeCalls++
	return nil
}

type testMFAStore struct{ closeCalls int }

func (s *testMFAStore) Save(string, string, time.Time) error { return nil }
func (s *testMFAStore) Get(string) (auth.MFAChallengeRecord, bool, error) {
	return auth.MFAChallengeRecord{}, false, nil
}
func (s *testMFAStore) Delete(string) error          { return nil }
func (s *testMFAStore) PurgeExpired(time.Time) error { return nil }
func (s *testMFAStore) Close(context.Context) error {
	s.closeCalls++
	return nil
}

func testServerRuntimeInput() ServerRuntimeInput {
	return ServerRuntimeInput{
		Logger:                 slog.New(slog.NewTextHandler(io.Discard, nil)),
		SessionStoreDriverFlag: "postgres",
		SessionTTL:             time.Hour,
		TracingProvider:        &tracing.Provider{},
	}
}

func TestNewServerRuntimeClosesStoreAndSessionOnMFAStoreFailure(t *testing.T) {
	origRepo := newPostgresRepository
	origSession := newPostgresSessionStore
	origMFA := newPostgresMFAChallengeStore
	origQueue := chatQueueFactory
	t.Cleanup(func() {
		newPostgresRepository = origRepo
		newPostgresSessionStore = origSession
		newPostgresMFAChallengeStore = origMFA
		chatQueueFactory = origQueue
	})

	repo := &testRepo{}
	session := &testSessionStore{}
	mfaErr := errors.New("mfa init boom")
	queueCalls := 0

	newPostgresRepository = func(string, ...storage.Option) (storage.Repository, error) { return repo, nil }
	newPostgresSessionStore = func(string, ...auth.PostgresSessionStoreOption) (closeableSessionStore, error) {
		return session, nil
	}
	newPostgresMFAChallengeStore = func(string, ...auth.PostgresMFAChallengeStoreOption) (closeableMFAStore, error) {
		return nil, mfaErr
	}
	chatQueueFactory = func(string, chat.RedisQueueConfig, *slog.Logger) (chat.Queue, error) {
		queueCalls++
		return nil, nil
	}

	_, err := NewServerRuntime(testServerRuntimeInput())
	if err == nil {
		t.Fatal("expected constructor error")
	}
	if !strings.Contains(err.Error(), "initialise mfa challenge store") {
		t.Fatalf("expected stage context in error, got %q", err)
	}
	if !errors.Is(err, mfaErr) {
		t.Fatalf("expected wrapped mfa error, got %v", err)
	}
	if repo.closeCalls != 1 {
		t.Fatalf("expected repo close once, got %d", repo.closeCalls)
	}
	if session.closeCalls != 1 {
		t.Fatalf("expected session close once, got %d", session.closeCalls)
	}
	if queueCalls != 0 {
		t.Fatalf("expected chat queue creation to be skipped, got %d calls", queueCalls)
	}
}

func TestNewServerRuntimeClosesCreatedStoresOnChatQueueFailure(t *testing.T) {
	origRepo := newPostgresRepository
	origSession := newPostgresSessionStore
	origMFA := newPostgresMFAChallengeStore
	origQueue := chatQueueFactory
	t.Cleanup(func() {
		newPostgresRepository = origRepo
		newPostgresSessionStore = origSession
		newPostgresMFAChallengeStore = origMFA
		chatQueueFactory = origQueue
	})

	repo := &testRepo{}
	session := &testSessionStore{}
	mfa := &testMFAStore{}
	queueErr := errors.New("queue init boom")

	newPostgresRepository = func(string, ...storage.Option) (storage.Repository, error) { return repo, nil }
	newPostgresSessionStore = func(string, ...auth.PostgresSessionStoreOption) (closeableSessionStore, error) {
		return session, nil
	}
	newPostgresMFAChallengeStore = func(string, ...auth.PostgresMFAChallengeStoreOption) (closeableMFAStore, error) {
		return mfa, nil
	}
	chatQueueFactory = func(string, chat.RedisQueueConfig, *slog.Logger) (chat.Queue, error) {
		return nil, queueErr
	}

	_, err := NewServerRuntime(testServerRuntimeInput())
	if err == nil {
		t.Fatal("expected constructor error")
	}
	if !strings.Contains(err.Error(), "initialise chat queue") {
		t.Fatalf("expected stage context in error, got %q", err)
	}
	if !errors.Is(err, queueErr) {
		t.Fatalf("expected wrapped queue error, got %v", err)
	}
	if repo.closeCalls != 1 {
		t.Fatalf("expected repo close once, got %d", repo.closeCalls)
	}
	if session.closeCalls != 1 {
		t.Fatalf("expected session close once, got %d", session.closeCalls)
	}
	if mfa.closeCalls != 1 {
		t.Fatalf("expected mfa close once, got %d", mfa.closeCalls)
	}
}
