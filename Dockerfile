# Multi-stage build for minimal final image
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . ./

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o gateway ./cmd/gateway

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy binary from builder
COPY --from=builder /app/gateway .

# Create data directory
RUN mkdir -p /data

# Expose ports
EXPOSE 18787 18788

# Set environment variables
ENV DATA_DIR=/data
ENV PROXY_PORT=18787
ENV MANAGEMENT_PORT=18788

CMD ["./gateway"]
