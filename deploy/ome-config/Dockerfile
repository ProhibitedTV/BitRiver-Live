# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM golang:1.26.5 AS builder
WORKDIR /src

ARG GOPROXY=direct
ARG GOSUMDB=off

ENV GOPROXY=$GOPROXY \
    GOSUMDB=$GOSUMDB \
    GOTOOLCHAIN=local \
    CGO_ENABLED=0 \
    GOFLAGS="-buildvcs=false"

COPY go.mod go.sum ./
COPY cmd/tools/production-module ./cmd/tools/production-module
COPY cmd/tools/verify-production-binary ./cmd/tools/verify-production-binary
COPY third_party ./third_party
RUN go run ./cmd/tools/production-module --output go.production.mod
RUN go mod download -modfile=go.production.mod all

COPY cmd ./cmd
COPY internal ./internal
COPY web ./web
COPY deploy ./deploy

RUN go build -modfile=go.production.mod -tags postgres -o /out/bitriver ./cmd/bitriver
RUN go run ./cmd/tools/verify-production-binary /out/bitriver

FROM scratch AS runner
WORKDIR /workspace

COPY --from=builder /out/bitriver /usr/local/bin/bitriver

ENTRYPOINT ["/usr/local/bin/bitriver"]
