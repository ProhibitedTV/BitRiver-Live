package pgx

import (
	"context"
	"errors"
)

type TxOptions struct{}

type Tx interface {
	Rollback(ctx context.Context) error
	Commit(ctx context.Context) error
	Exec(ctx context.Context, sql string, args ...interface{}) (CommandTag, error)
	Query(ctx context.Context, sql string, args ...interface{}) (Rows, error)
	QueryRow(ctx context.Context, sql string, args ...interface{}) Row
}

type Row interface {
	Scan(dest ...interface{}) error
}

type Rows interface {
	Close()
	Next() bool
	Scan(dest ...interface{}) error
	Err() error
}

type CommandTag struct{}

func (CommandTag) RowsAffected() int64 { return 0 }

var (
	ErrNoRows   = errors.New("pgx: no rows in result set")
	ErrTxClosed = errors.New("pgx: tx closed")
)
