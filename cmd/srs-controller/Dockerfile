# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM golang:1.26.5-alpine AS build
WORKDIR /src

RUN apk add --no-cache ca-certificates git openssh-client

ARG TARGETOS=linux
ARG TARGETARCH
ARG GOPROXY=direct
ARG GOSUMDB=off

COPY go.mod go.sum ./
COPY cmd/tools/production-module ./cmd/tools/production-module
COPY cmd/tools/verify-production-binary ./cmd/tools/verify-production-binary
COPY third_party ./third_party
RUN go run ./cmd/tools/production-module --output go.production.mod
RUN GOPROXY=$GOPROXY GOSUMDB=$GOSUMDB go mod download -modfile=go.production.mod all

COPY internal ./internal
COPY cmd ./cmd
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -modfile=go.production.mod -o /out/srs-controller ./cmd/srs-controller
RUN go run ./cmd/tools/verify-production-binary /out/srs-controller

FROM --platform=$TARGETPLATFORM alpine:3.23 AS runtime
RUN apk add --no-cache ca-certificates curl

WORKDIR /app
COPY --from=build /out/srs-controller /usr/local/bin/srs-controller

RUN adduser -D -u 10002 appuser && mkdir -p /var/lib/srs-controller && chown appuser /var/lib/srs-controller

ENV SRS_CONTROLLER_BIND=:1985
ENV SRS_CONTROLLER_UPSTREAM=http://srs:1985/api/

USER appuser
EXPOSE 1985

ENTRYPOINT ["/usr/local/bin/srs-controller"]
