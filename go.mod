module bitriver-live

go 1.21

require (
        github.com/jackc/pgx/v5 v5.5.4
        github.com/jackc/puddle/v2 v2.2.1
        github.com/redis/go-redis/v9 v9.0.0
        golang.org/x/crypto v0.17.0
)

require (
        github.com/jackc/pgpassfile v1.0.0 // indirect
        github.com/jackc/pgservicefile v0.0.0-20221227161230-091c0ba34f0a // indirect
        golang.org/x/sync v0.1.0 // indirect
        golang.org/x/text v0.14.0 // indirect
)

replace golang.org/x/crypto => ./third_party/golang.org/x/crypto

replace github.com/jackc/pgx/v5 => ./third_party/github.com/jackc/pgx/v5

replace github.com/jackc/puddle/v2 => ./third_party/github.com/jackc/puddle/v2

replace github.com/redis/go-redis/v9 => ./third_party/github.com/redis/go-redis/v9
