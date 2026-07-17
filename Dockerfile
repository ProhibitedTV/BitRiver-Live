# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM golang:1.26.5 AS builder
WORKDIR /src

ARG TARGETOS=linux
ARG TARGETARCH
ARG GOPROXY=direct
ARG GOSUMDB=off
ARG BITRIVER_PGX_MODE=real

ENV CGO_ENABLED=0 GOFLAGS="-buildvcs=false"
ENV GOPROXY=$GOPROXY GOSUMDB=$GOSUMDB

ENV GOOS=$TARGETOS
ENV GOARCH=$TARGETARCH

COPY go.mod go.sum ./
COPY third_party ./third_party
COPY cmd/tools/production-module ./cmd/tools/production-module
RUN test "$BITRIVER_PGX_MODE" = "real"
RUN go run ./cmd/tools/production-module --output go.production.mod
RUN GOPROXY=$GOPROXY GOSUMDB=$GOSUMDB go mod download -modfile=go.production.mod all

COPY cmd ./cmd
COPY internal ./internal
COPY web ./web
COPY deploy/migrations ./deploy/migrations
COPY scripts/check-postgres-pgx.sh ./scripts/check-postgres-pgx.sh
COPY cmd/tools/verify-production-binary ./cmd/tools/verify-production-binary

RUN GOFLAGS="-buildvcs=false -modfile=/src/go.production.mod" ./scripts/check-postgres-pgx.sh postgres
RUN go build -modfile=go.production.mod -tags postgres -o /out/bitriver-live ./cmd/server
RUN go build -modfile=go.production.mod -tags postgres -o /out/bootstrap-admin ./cmd/tools/bootstrap-admin
RUN go run ./cmd/tools/verify-production-binary --require-module github.com/jackc/pgx/v5 /out/bitriver-live
RUN go run ./cmd/tools/verify-production-binary --require-module github.com/jackc/pgx/v5 /out/bootstrap-admin

FROM alpine:3.23 AS runtime
RUN apk add --no-cache ca-certificates curl

WORKDIR /app

COPY --from=builder /out/bitriver-live /app/bitriver-live
COPY --from=builder /out/bootstrap-admin /app/bootstrap-admin
COPY --from=builder /src/deploy/migrations /app/deploy/migrations

RUN adduser -D -H -u 65532 appuser && chown appuser /app

USER appuser

EXPOSE 8080
ENTRYPOINT ["/app/bitriver-live"]
