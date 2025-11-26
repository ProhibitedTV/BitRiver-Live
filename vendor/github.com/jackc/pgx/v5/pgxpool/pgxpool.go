//go:build !postgres

package pgxpool

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type Config struct {
	ConnConfig        *pgx.ConnConfig
	MaxConns          int32
	MinConns          int32
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
}

func ParseConfig(dsn string) (*Config, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, errors.New("pgxpool: dsn required")
	}
	cfg := &Config{
		ConnConfig: &pgx.ConnConfig{RuntimeParams: map[string]string{}},
	}
	return cfg, nil
}

func NewWithConfig(ctx context.Context, cfg *Config) (*Pool, error) {
	if cfg == nil {
		return nil, errors.New("pgxpool: config required")
	}
	if cfg.ConnConfig == nil {
		cfg.ConnConfig = &pgx.ConnConfig{RuntimeParams: map[string]string{}}
	}
	return &Pool{cfg: cfg}, nil
}

type Pool struct {
	cfg *Config
}

func (p *Pool) Close() {}

func (p *Pool) Acquire(ctx context.Context) (*Conn, error) {
	if p == nil {
		return nil, errors.New("pgxpool: pool is nil")
	}
	return &Conn{pool: p}, nil
}

func (p *Pool) BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error) {
	if p == nil {
		return nil, errors.New("pgxpool: pool is nil")
	}
	return &stubTx{}, nil
}

func (p *Pool) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return stubRow{}
}

func (p *Pool) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return &stubRows{}, nil
}

func (p *Pool) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

type Conn struct {
	pool *Pool
}

func (c *Conn) Release() {}

func (c *Conn) BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error) {
	if c == nil {
		return nil, errors.New("pgxpool: conn is nil")
	}
	return &stubTx{}, nil
}

func (c *Conn) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return stubRow{}
}

func (c *Conn) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return &stubRows{}, nil
}

func (c *Conn) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

type stubRow struct {
	err error
}

func (r stubRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	return pgx.ErrNoRows
}

type stubRows struct {
	err error
}

func (r *stubRows) Close() {}

func (r *stubRows) Next() bool { return false }

func (r *stubRows) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	return pgx.ErrNoRows
}

func (r *stubRows) Err() error { return r.err }

type stubTx struct {
	closed bool
}

func (tx *stubTx) Commit(ctx context.Context) error {
	if tx.closed {
		return pgx.ErrTxClosed
	}
	tx.closed = true
	return nil
}

func (tx *stubTx) Rollback(ctx context.Context) error {
	if tx.closed {
		return nil
	}
	tx.closed = true
	return nil
}

func (tx *stubTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if tx.closed {
		return pgconn.CommandTag{}, pgx.ErrTxClosed
	}
	return pgconn.CommandTag{}, nil
}

func (tx *stubTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if tx.closed {
		return stubRow{err: pgx.ErrTxClosed}
	}
	return stubRow{}
}

func (tx *stubTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if tx.closed {
		return nil, pgx.ErrTxClosed
	}
	return &stubRows{}, nil
}
