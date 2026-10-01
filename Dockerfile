# Build with the same Go toolchain as CI and the deploy build (patched stdlib);
# go.mod's go directive (1.25) is only the language version.
FROM golang:1.26.4-alpine AS builder
WORKDIR /app
RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown
RUN CGO_ENABLED=0 GOOS=linux go build \
      -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildTime=${BUILD_TIME}" \
      -o parashift ./cmd/server

FROM alpine:3.20
RUN apk --no-cache add ca-certificates tzdata \
    # Create a non-root user so the process cannot write to the container FS
    # or escalate privileges if the binary is ever compromised.
 && addgroup -S app \
 && adduser  -S -G app app

WORKDIR /app
COPY --from=builder /app/parashift .
COPY --from=builder /app/migrations ./migrations

# Drop to non-root before the process starts.
USER app

# PORT defaults to 4000 (internal/config). In a container the API must listen
# on every interface; on the VPS it binds 127.0.0.1 behind Caddy.
ENV PORT=4000 \
    BIND_ADDR=0.0.0.0
EXPOSE 4000
ENTRYPOINT ["./parashift"]
