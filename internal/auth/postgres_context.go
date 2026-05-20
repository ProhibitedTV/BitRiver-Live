package auth

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func postgresOperationContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout > 0 {
		return context.WithTimeout(context.Background(), timeout)
	}
	return context.Background(), func() {}
}

func pingPostgresPool(ctx context.Context, pool *pgxpool.Pool, timeout time.Duration) error {
	ctx = normalizeContext(ctx)
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	_, execErr := conn.Exec(ctx, "SELECT 1")
	return execErr
}
