# Multi-stage Dockerfile for building Ducke Linux/SteamOS binary
FROM golang:1.24-bookworm AS builder

# Install system dependencies for Wails on Linux (WebKitGTK + GTK3)
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    pkg-config \
    libgtk-3-dev \
    libwebkit2gtk-4.1-dev \
    curl \
    git \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Install Node.js (v20 LTS)
RUN curl -fsSL https://deb.nodesource.com/setup_20.x | bash - && \
    apt-get install -y nodejs && \
    rm -rf /var/lib/apt/lists/*

# Install Wails v2 CLI
RUN go install github.com/wailsapp/wails/v2/cmd/wails@latest

WORKDIR /app

# Copy go mod and npm dependencies first for caching
COPY go.mod go.sum ./
RUN go mod download

COPY frontend/package*.json ./frontend/
RUN cd frontend && npm install

# Copy entire source
COPY . .

# Build Linux/amd64 production binary
RUN wails build -platform linux/amd64 -clean -v 2

# Output stage
FROM scratch AS export
COPY --from=builder /app/build/bin/Ducke /Ducke
