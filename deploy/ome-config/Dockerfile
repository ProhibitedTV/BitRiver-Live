# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM golang:1.21 AS builder
WORKDIR /src

ARG GOPROXY=off
ARG GOSUMDB=off

ENV GOPROXY=$GOPROXY \
    GOSUMDB=$GOSUMDB \
    GOTOOLCHAIN=local \
    CGO_ENABLED=0 \
    GOFLAGS="-buildvcs=false"

COPY go.mod go.sum ./
COPY cmd ./cmd
COPY internal ./internal
COPY third_party ./third_party
COPY web ./web
COPY deploy ./deploy

RUN go build -tags postgres -o /out/bitriver ./cmd/bitriver

FROM scratch AS runner
WORKDIR /workspace

COPY --from=builder /out/bitriver /usr/local/bin/bitriver

ENTRYPOINT ["/usr/local/bin/bitriver"]
