package pgxpool

import (
	"context"
	"time"

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

type Pool struct{}

func ParseConfig(string) (*Config, error) {
	return &Config{ConnConfig: &ConnConfig{RuntimeParams: make(map[string]string)}}, nil
}

func NewWithConfig(context.Context, *Config) (*Pool, error) {
	return &Pool{}, nil
}

func (p *Pool) Close() {}

func (p *Pool) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
