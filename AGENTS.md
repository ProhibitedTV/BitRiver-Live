# Repository Guidelines

- Format Go source with `gofmt` before committing.
- When running tests, set the following environment variables to avoid network lookups: `GOTOOLCHAIN=local`, `GOPROXY=off`, `GOSUMDB=off`. Run the full suite with `go test ./... -count=1 -timeout=10s`.
- Prefer small, focused helpers over duplicated logic. When touching persistence code, keep writes atomic and avoid introducing non-deterministic behaviour in tests.
- Update or add tests alongside behavioural changes and keep their runtime under a few hundred milliseconds.
