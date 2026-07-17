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
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -modfile=go.production.mod -o /out/transcoder ./cmd/transcoder
RUN go run ./cmd/tools/verify-production-binary /out/transcoder

FROM alpine:3.23 AS ffmpeg
RUN apk add --no-cache ffmpeg

FROM --platform=$TARGETPLATFORM alpine:3.23 AS runtime
RUN apk add --no-cache ca-certificates curl

WORKDIR /app
COPY --from=build /out/transcoder /usr/local/bin/transcoder
COPY --from=ffmpeg /usr/bin/ffmpeg /usr/local/bin/ffmpeg
COPY --from=ffmpeg /usr/lib/ /usr/lib/
COPY --from=ffmpeg /lib/ /lib/

RUN adduser -D -u 10001 appuser && mkdir -p /work && chown appuser /work

ENV JOB_CONTROLLER_BIND=:9000
ENV JOB_CONTROLLER_OUTPUT_ROOT=/work

USER appuser
EXPOSE 9000

ENTRYPOINT ["/usr/local/bin/transcoder"]
