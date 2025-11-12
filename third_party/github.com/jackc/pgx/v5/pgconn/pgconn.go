//go:build !postgres

package pgconn

type CommandTag struct {
	rowsAffected int64
}

func (c CommandTag) RowsAffected() int64 {
	return c.rowsAffected
}
