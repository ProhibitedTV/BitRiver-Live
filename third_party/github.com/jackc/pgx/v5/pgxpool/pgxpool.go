package pgxpool

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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

var ErrClosedPool = errors.New("pgxpool stub: pool closed")

type Pool struct{}

type Row struct{}

func (r *Row) Scan(dest ...any) error {
	return errors.New("pgxpool stub: no rows")
}

type Rows struct{}

func (r *Rows) Close() {}

func (r *Rows) Next() bool {
	return false
}

func (r *Rows) Scan(dest ...any) error {
	return errors.New("pgxpool stub: no rows")
}

func (r *Rows) Err() error {
	return nil
}

type Tx struct{}

func (tx *Tx) Commit(context.Context) error {
	return nil
}

func (tx *Tx) Rollback(context.Context) error {
	return nil
}

func (tx *Tx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (tx *Tx) QueryRow(context.Context, string, ...any) pgx.Row {
	return &Row{}
}

func (tx *Tx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return &Rows{}, nil
}

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

func (p *Pool) QueryRow(context.Context, string, ...any) pgx.Row {
	return &Row{}
}

func (p *Pool) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return &Rows{}, nil
}

func (p *Pool) BeginTx(context.Context, pgx.TxOptions) (*Tx, error) {
	return &Tx{}, nil
}
