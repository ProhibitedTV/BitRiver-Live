package pgx

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrNoRows   = errors.New("pgx stub: no rows")
	ErrTxClosed = errors.New("pgx stub: transaction closed")
)

type TxOptions struct{}

type Row interface {
	Scan(dest ...any) error
}

type Rows interface {
	Close()
	Next() bool
	Scan(dest ...any) error
	Err() error
}

type Tx interface {
	Commit(context.Context) error
	Rollback(context.Context) error
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	QueryRow(context.Context, string, ...any) Row
	Query(context.Context, string, ...any) (Rows, error)
}
