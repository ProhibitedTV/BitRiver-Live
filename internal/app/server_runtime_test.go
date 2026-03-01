package app

import (
	"bytes"
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
	"bitriver-live/internal/server"
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

type testErrorRepo struct {
	storage.Repository
	closeErr error
	onClose  func()
}

func (r *testErrorRepo) Close(context.Context) error {
	if r.onClose != nil {
		r.onClose()
	}
	return r.closeErr
}

func TestServerRuntimeShutdownLogsCloseWarningsAndContinues(t *testing.T) {
	ctx := context.Background()
	var logOutput bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logOutput, nil))

	storeErr := errors.New("store close failed")
	sessionErr := errors.New("session close failed")
	mfaErr := errors.New("mfa close failed")

	closeOrder := make([]string, 0, 3)
	runtime := &ServerRuntime{
		server: &server.Server{},
		logger: logger,
		store: &testErrorRepo{
			closeErr: storeErr,
			onClose: func() {
				closeOrder = append(closeOrder, "store")
			},
		},
		sessionCloser: func(context.Context) error {
			closeOrder = append(closeOrder, "session_store")
			return sessionErr
		},
		mfaCloser: func(context.Context) error {
			closeOrder = append(closeOrder, "mfa_store")
			return mfaErr
		},
	}

	runtime.Shutdown(ctx)

	if got, want := strings.Join(closeOrder, ","), "store,session_store,mfa_store"; got != want {
		t.Fatalf("expected close order %q, got %q", want, got)
	}

	logs := logOutput.String()
	if !strings.Contains(logs, "failed to close store") || !strings.Contains(logs, "component=store") || !strings.Contains(logs, storeErr.Error()) {
		t.Fatalf("expected store warning log, got %q", logs)
	}
	if !strings.Contains(logs, "failed to close session store") || !strings.Contains(logs, "component=session_store") || !strings.Contains(logs, sessionErr.Error()) {
		t.Fatalf("expected session warning log, got %q", logs)
	}
	if !strings.Contains(logs, "failed to close mfa store") || !strings.Contains(logs, "component=mfa_store") || !strings.Contains(logs, mfaErr.Error()) {
		t.Fatalf("expected mfa warning log, got %q", logs)
	}
}
