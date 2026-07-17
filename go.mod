module bitriver-live

go 1.26.0

require (
	github.com/jackc/pgpassfile v1.0.0
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761
	github.com/jackc/pgx/v5 v5.10.0
	github.com/jackc/puddle/v2 v2.2.2
	github.com/redis/go-redis/v9 v9.21.0
	golang.org/x/crypto v0.54.0
	golang.org/x/sync v0.22.0
	golang.org/x/text v0.40.0
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
)

replace golang.org/x/crypto => ./third_party/golang.org/x/crypto

replace github.com/jackc/pgx/v5 => ./third_party/github.com/jackc/pgx/v5

replace github.com/jackc/puddle/v2 => ./third_party/github.com/jackc/puddle/v2

replace github.com/redis/go-redis/v9 => ./third_party/github.com/redis/go-redis/v9

replace github.com/jackc/pgpassfile => ./third_party/github.com/jackc/pgpassfile

replace github.com/jackc/pgservicefile => ./third_party/github.com/jackc/pgservicefile

replace golang.org/x/sync => ./third_party/golang.org/x/sync

replace golang.org/x/text => ./third_party/golang.org/x/text
