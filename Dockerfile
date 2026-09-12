# Multi-stage build for minimal image size

# Stage 1: Build backend
FROM node:20-alpine AS backend-builder
WORKDIR /app/backend
COPY backend/package*.json ./
RUN npm ci --production=false
COPY backend/ ./
RUN npm run build

# Stage 2: Build frontend
FROM node:20-alpine AS frontend-builder
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# Stage 3: Production image
FROM node:20-alpine
LABEL maintainer="Redact Gateway"
LABEL description="Privacy-first redaction gateway for AI APIs"

# Install production dependencies
WORKDIR /app
COPY backend/package*.json ./
RUN npm ci --production && npm cache clean --force

# Copy built artifacts
COPY --from=backend-builder /app/backend/dist ./dist
COPY --from=frontend-builder /app/frontend/dist ./dist/public

# Create data directory
RUN mkdir -p /data && chown -R node:node /data

# Switch to non-root user
USER node

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
  CMD node -e "require('http').get('http://localhost:18788/api/status', (r) => { process.exit(r.statusCode === 200 ? 0 : 1); })"

EXPOSE 18787 18788

CMD ["node", "dist/main.js"]
