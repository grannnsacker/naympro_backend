# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install migrate
RUN apk add --no-cache curl
RUN curl -L https://github.com/golang-migrate/migrate/releases/download/v4.16.2/migrate.linux-amd64.tar.gz -o migrate.tar.gz && \
    tar xf migrate.tar.gz && \
    mv migrate /usr/local/bin/migrate && \
    rm migrate.tar.gz

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o main ./cmd/main.go

# Final stage
FROM alpine:latest

WORKDIR /app

# Install curl in the final stage
RUN apk add --no-cache curl

# Download and install migrate in the final stage
RUN curl -L https://github.com/golang-migrate/migrate/releases/download/v4.16.2/migrate.linux-amd64.tar.gz -o migrate.tar.gz && \
    tar xf migrate.tar.gz && \
    mv migrate /usr/local/bin/migrate && \
    chmod +x /usr/local/bin/migrate && \
    rm migrate.tar.gz

# Copy the binary and migrations from builder
COPY --from=builder /app/main .
COPY --from=builder /app/app.env .
COPY --from=builder /app/internal/db/migrations ./internal/db/migrations

# Expose port
EXPOSE 8080

# Run the application
CMD ["./main"]