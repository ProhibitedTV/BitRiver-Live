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

FROM --platform=$TARGETPLATFORM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/transcoder /app/transcoder

ENV JOB_CONTROLLER_BIND=:9000
EXPOSE 9000

ENTRYPOINT ["/app/transcoder"]
