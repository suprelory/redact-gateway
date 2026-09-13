# syntax=docker/dockerfile:1.7

FROM node:24-alpine AS web-builder
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.24-alpine AS go-builder
ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web-builder /src/internal/admin/web/dist ./internal/admin/web/dist
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
    -trimpath \
    -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildTime=${BUILD_TIME}" \
    -o /out/redact-gateway ./cmd/redact-gateway

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S -g 10001 redact \
    && adduser -S -D -H -u 10001 -G redact redact \
    && mkdir -p /data \
    && chown -R redact:redact /data
COPY --from=go-builder /out/redact-gateway /usr/local/bin/redact-gateway

USER 10001:10001
WORKDIR /data
ENV REDACT_LISTEN_ADDR=0.0.0.0:8787 \
    REDACT_ADMIN_ADDR=0.0.0.0:8788 \
    REDACT_DATA_DIR=/data
EXPOSE 8787 8788
VOLUME ["/data"]
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8787/healthz >/dev/null || exit 1
ENTRYPOINT ["/usr/local/bin/redact-gateway"]
CMD ["serve"]
