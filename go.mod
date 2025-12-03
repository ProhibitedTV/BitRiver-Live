module bitriver-live

go 1.21

require (
	github.com/jackc/pgpassfile v1.0.0
	github.com/jackc/pgservicefile v0.0.0-20221227161230-091c0ba34f0a
	github.com/jackc/pgx/v5 v5.7.4
	github.com/jackc/puddle/v2 v2.3.2
	github.com/redis/go-redis/v9 v9.5.1
	golang.org/x/crypto v0.27.0
	golang.org/x/sync v0.7.0
	golang.org/x/text v0.18.0
)

replace golang.org/x/crypto => ./third_party/golang.org/x/crypto

replace github.com/jackc/pgx/v5 => ./third_party/github.com/jackc/pgx/v5

replace github.com/jackc/puddle/v2 => ./third_party/github.com/jackc/puddle/v2

replace github.com/redis/go-redis/v9 => ./third_party/github.com/redis/go-redis/v9

replace github.com/jackc/pgpassfile => ./third_party/github.com/jackc/pgpassfile

replace github.com/jackc/pgservicefile => ./third_party/github.com/jackc/pgservicefile

replace golang.org/x/sync => ./third_party/golang.org/x/sync

replace golang.org/x/text => ./third_party/golang.org/x/text
