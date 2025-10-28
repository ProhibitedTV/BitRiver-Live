package storage

import "fmt"

// ErrPostgresUnavailable is returned when the Postgres repository has not yet
// been wired into the build.
var ErrPostgresUnavailable = fmt.Errorf("postgres repository unavailable")

// NewPostgresRepository returns an error until the Postgres implementation is
// provided. The signature exists so dependency injection can be configured
// ahead of time.
func NewPostgresRepository(dsn string, opts ...Option) (Repository, error) {
	cfg := newPostgresConfig(dsn, opts...)
	_ = cfg
	return nil, ErrPostgresUnavailable
}
