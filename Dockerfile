FROM golang:1.22-alpine AS builder
WORKDIR /app
RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o parashift ./cmd/server

FROM alpine:3.19
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

EXPOSE 8080
ENTRYPOINT ["./parashift"]
