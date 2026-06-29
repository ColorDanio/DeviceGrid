# syntax=docker/dockerfile:1.6

# ---------- Stage 1: Build frontend ----------
FROM node:22-alpine AS web-builder
WORKDIR /web
COPY web/package*.json ./
RUN npm ci --no-audit --no-fund
COPY web/ ./
RUN npm run build

# ---------- Stage 2: Build server + agent ----------
FROM golang:1.22-alpine AS go-builder
ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG BUILD_DATE=unknown
ENV CGO_ENABLED=0 GOOS=linux

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY --from=web-builder /web/dist ./web/dist
COPY . .

RUN go build -trimpath \
    -ldflags="-s -w -X github.com/michael/device_grid/internal/version.Version=${VERSION} -X github.com/michael/device_grid/internal/version.GitCommit=${GIT_COMMIT} -X github.com/michael/device_grid/internal/version.BuildDate=${BUILD_DATE}" \
    -o /out/devicegrid-server ./cmd/server

RUN GOARCH=amd64 go build -trimpath \
    -ldflags="-s -w -X github.com/michael/device_grid/internal/version.Version=${VERSION} -X github.com/michael/device_grid/internal/version.GitCommit=${GIT_COMMIT}" \
    -o /out/devicegrid-agent-linux-amd64 ./cmd/agent

RUN GOARCH=arm64 go build -trimpath \
    -ldflags="-s -w -X github.com/michael/device_grid/internal/version.Version=${VERSION} -X github.com/michael/device_grid/internal/version.GitCommit=${GIT_COMMIT}" \
    -o /out/devicegrid-agent-linux-arm64 ./cmd/agent

# ---------- Stage 3: Runtime ----------
FROM gcr.io/distroless/static-debian12:nonroot
LABEL org.opencontainers.image.title="DeviceGrid" \
      org.opencontainers.image.description="Server fleet management dashboard" \
      org.opencontainers.image.source="https://github.com/michael/device_grid"

WORKDIR /app
COPY --from=go-builder /out/devicegrid-server /app/devicegrid-server
COPY --from=go-builder /out/devicegrid-agent-linux-amd64 /app/devicegrid-agent-linux-amd64
COPY --from=go-builder /out/devicegrid-agent-linux-arm64 /app/devicegrid-agent-linux-arm64
COPY configs/ /app/configs/

ENV DG_CONFIG=/app/configs/config.yaml
ENV DG_DATA_DIR=/app/data
EXPOSE 3000 9090
USER nonroot:nonroot

VOLUME ["/app/data", "/app/configs/certs"]
ENTRYPOINT ["/app/devicegrid-server"]