# Stage 1: Build the binary
FROM golang:1.25-alpine AS builder
WORKDIR /app

# Install dependencies for CGO + sqlite3
RUN apk add --no-cache git gcc musl-dev
COPY . .
ENV CGO_ENABLED=1
RUN go build -o /rdf-exporter ./main.go

# Stage 2: Minimal runtime image
FROM alpine:3.20

# Install ca-certificates + busybox cron
RUN apk add --no-cache ca-certificates bash busybox-extras

# Copy the built binary
COPY --from=builder /rdf-exporter /usr/local/bin/rdf-exporter

# Create directories for DB and exports
#RUN mkdir -p /data /exports

COPY crontab /etc/crontabs/root

CMD ["crond", "-f", "-L", "/dev/stdout"]
