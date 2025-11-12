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
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
COPY web ./web
COPY deploy/migrations ./deploy/migrations

RUN go build -o /out/bitriver-live ./cmd/server

FROM --platform=$TARGETPLATFORM gcr.io/distroless/base-debian12:nonroot
WORKDIR /app

COPY --from=builder /out/bitriver-live /app/bitriver-live
COPY --from=builder /src/deploy/migrations /app/deploy/migrations

EXPOSE 8080
ENTRYPOINT ["/app/bitriver-live"]
