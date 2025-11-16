# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM golang:1.21 AS builder
WORKDIR /src

ARG TARGETOS=linux
ARG TARGETARCH

ENV CGO_ENABLED=0 GOFLAGS=-buildvcs=false

ENV GOOS=$TARGETOS
ENV GOARCH=$TARGETARCH

COPY go.mod go.sum ./
COPY third_party ./third_party
RUN go mod edit -dropreplace github.com/jackc/pgx/v5 \
    && go mod edit -dropreplace github.com/jackc/puddle/v2 \
    && go mod download

COPY cmd ./cmd
COPY internal ./internal
COPY web ./web
COPY deploy/migrations ./deploy/migrations

RUN go build -tags postgres -o /out/bitriver-live ./cmd/server
RUN go build -tags postgres -o /out/bootstrap-admin ./cmd/tools/bootstrap-admin

FROM --platform=$TARGETPLATFORM debian:12-slim AS runtime
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /out/bitriver-live /app/bitriver-live
COPY --from=builder /out/bootstrap-admin /app/bootstrap-admin
COPY --from=builder /src/deploy/migrations /app/deploy/migrations

RUN useradd -r -u 65532 appuser && chown appuser /app

USER appuser

EXPOSE 8080
ENTRYPOINT ["/app/bitriver-live"]
