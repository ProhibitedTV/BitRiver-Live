//go:build postgres

package storage

// This file exists solely to pin transitive dependencies that are required when
// building the storage package with the "postgres" build tag. The real
// implementations live in the upstream pgx module, which is swapped in by the
// test harness when running postgres-tagged integration tests. Keeping these
// blank imports ensures the go tool recognises the dependencies when tidying
// modules in this repository.
import (
	_ "github.com/jackc/pgpassfile"
	_ "github.com/jackc/pgservicefile"
	_ "golang.org/x/sync/semaphore"
	_ "golang.org/x/text/transform"
)
