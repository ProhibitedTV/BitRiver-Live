# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM golang:1.21-alpine AS build
WORKDIR /src

RUN apk add --no-cache ca-certificates

ARG TARGETOS=linux
ARG TARGETARCH

COPY go.mod go.sum ./
COPY internal ./internal
COPY cmd ./cmd
COPY third_party ./third_party

RUN go mod download
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /out/transcoder ./cmd/transcoder

FROM alpine:3.19 AS ffmpeg
RUN apk add --no-cache ffmpeg

FROM --platform=$TARGETPLATFORM alpine:3.19 AS runtime
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
