module bitriver-live

go 1.21

require (
        github.com/jackc/pgx/v5 v5.5.4
        golang.org/x/crypto v0.0.0
)

replace golang.org/x/crypto => ./third_party/golang.org/x/crypto
replace github.com/jackc/pgx/v5 => ./third_party/github.com/jackc/pgx/v5
