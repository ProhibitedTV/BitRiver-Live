package pgxpool

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

type Config struct {
	MaxConns          int32
	MinConns          int32
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
	ConnConfig        *ConnConfig
}

type ConnConfig struct {
	ConnectTimeout time.Duration
	RuntimeParams  map[string]string
}

type Pool struct{}

func ParseConfig(string) (*Config, error) {
	return &Config{ConnConfig: &ConnConfig{RuntimeParams: make(map[string]string)}}, nil
}

func NewWithConfig(context.Context, *Config) (*Pool, error) {
	return &Pool{}, nil
}

func (p *Pool) Close() {}

func (p *Pool) Exec(context.Context, string, ...any) (pgx.CommandTag, error) {
	return pgx.CommandTag{}, nil
}

func (p *Pool) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	return &noopTx{}, nil
}

func (p *Pool) QueryRow(context.Context, string, ...any) pgx.Row {
	return noopRow{}
}

func (p *Pool) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return &noopRows{}, nil
}

type noopTx struct{}

func (n *noopTx) Rollback(context.Context) error { return nil }

func (n *noopTx) Commit(context.Context) error { return nil }

func (n *noopTx) Exec(context.Context, string, ...any) (pgx.CommandTag, error) {
	return pgx.CommandTag{}, nil
}

func (n *noopTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return &noopRows{}, nil
}

func (n *noopTx) QueryRow(context.Context, string, ...any) pgx.Row {
	return noopRow{}
}

type noopRow struct{}

func (noopRow) Scan(dest ...interface{}) error { return pgx.ErrNoRows }

type noopRows struct{}

func (r *noopRows) Close() {}

func (r *noopRows) Next() bool { return false }

func (r *noopRows) Scan(dest ...interface{}) error { return pgx.ErrNoRows }

func (r *noopRows) Err() error { return nil }
