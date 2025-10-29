module bitriver-live

go 1.21

require (
	github.com/jackc/pgx/v5 v5.5.4
	github.com/redis/go-redis/v9 v9.0.0
	golang.org/x/crypto v0.17.0
)

replace golang.org/x/crypto => ./third_party/golang.org/x/crypto

replace github.com/redis/go-redis/v9 => ./third_party/github.com/redis/go-redis/v9
