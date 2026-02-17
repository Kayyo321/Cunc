#!/bin/bash

# Build Cunc binaries for Windows, Linux, and macOS
# Outputs to ./bin/release
# Note: Builds without cgo for cross-platform compatibility

set -e

RELEASE_DIR="./bin/release"
BINARY_NAME="cunc"
VERSION=$(git describe --tags --always 2>/dev/null || echo "dev")

# Create release directory
mkdir -p "$RELEASE_DIR"

echo "Building Cunc v$VERSION for multiple platforms..."
echo ""

# Linux amd64
echo "Building for Linux (amd64)..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o "$RELEASE_DIR/${BINARY_NAME}-linux-amd64" ./src/cunc.go

# Linux arm64
echo "Building for Linux (arm64)..."
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o "$RELEASE_DIR/${BINARY_NAME}-linux-arm64" ./src/cunc.go

# macOS amd64
echo "Building for macOS (amd64)..."
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o "$RELEASE_DIR/${BINARY_NAME}-macos-amd64" ./src/cunc.go

# macOS arm64 (Apple Silicon)
echo "Building for macOS (arm64)..."
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o "$RELEASE_DIR/${BINARY_NAME}-macos-arm64" ./src/cunc.go

# Windows amd64
echo "Building for Windows (amd64)..."
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o "$RELEASE_DIR/${BINARY_NAME}-windows-amd64.exe" ./src/cunc.go

# Windows arm64
echo "Building for Windows (arm64)..."
CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -o "$RELEASE_DIR/${BINARY_NAME}-windows-arm64.exe" ./src/cunc.go

echo ""
echo "Build complete! Binaries are in $RELEASE_DIR:"
ls -lh "$RELEASE_DIR"
