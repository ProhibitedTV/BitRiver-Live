# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM golang:1.21-alpine AS build
WORKDIR /src

RUN apk add --no-cache ca-certificates

ARG TARGETOS=linux
ARG TARGETARCH
ARG GOPROXY=direct
ARG GOSUMDB=off

COPY go.mod go.sum ./
COPY internal ./internal
COPY cmd ./cmd
COPY third_party ./third_party

RUN GOPROXY=$GOPROXY GOSUMDB=$GOSUMDB go mod download
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /out/srs-controller ./cmd/srs-controller

FROM --platform=$TARGETPLATFORM alpine:3.19 AS runtime
RUN apk add --no-cache ca-certificates curl

WORKDIR /app
COPY --from=build /out/srs-controller /usr/local/bin/srs-controller

RUN adduser -D -u 10002 appuser && mkdir -p /var/lib/srs-controller && chown appuser /var/lib/srs-controller

ENV SRS_CONTROLLER_BIND=:1985
ENV SRS_CONTROLLER_UPSTREAM=http://srs:1985/api/

USER appuser
EXPOSE 1985

ENTRYPOINT ["/usr/local/bin/srs-controller"]
