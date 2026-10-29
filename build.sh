#!/usr/bin/env bash

# Exit immediately if a command exits with a non-zero status
set -e

# --- Configuration ---
BINARY_NAME="lan-p2p"
DIST_DIR="dist"
VERSION=${1:-"1.0.0"}
BUILD_TIME=$(date -u +'%Y-%m-%dT%H:%M:%SZ')
COMMIT_HASH=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Define target package path for ldflags variables
# Change 'main' if your global version variables are defined in another package (e.g., 'lan-p2p/version')
LDFLAGS="-X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME} -X main.CommitHash=${COMMIT_HASH} -s -w"

# --- Target Matrix ---
# Format: "GOOS/GOARCH"
TARGETS=(
    "linux/amd64"
    "linux/arm64"
    "windows/amd64"
    "darwin/amd64"
    "darwin/arm64"
)

# --- 1. Clean Step ---
echo "🧹 Cleaning previous builds..."
rm -rf "${DIST_DIR}"
mkdir -p "${DIST_DIR}"

# --- 2. Build Step ---
echo "🚀 Starting compilation for version: ${VERSION} (Commit: ${COMMIT_HASH})..."

for TARGET in "${TARGETS[@]}"; do
    # Split the target into OS and ARCH
    IFS="/" read -r GOOS GOARCH <<< "${TARGET}"
    
    # Define output filename (add .exe extension for Windows)
    OUTPUT_NAME="${DIST_DIR}/${BINARY_NAME}-${GOOS}-${GOARCH}"
    if [ "$GOOS" = "windows" ]; then
        OUTPUT_NAME="${OUTPUT_NAME}.exe"
    fi
    
    echo "  📦 Building for ${GOOS}/${GOARCH}..."
    
    # Run the cross-compilation
    CGO_ENABLED=0 GOOS=${GOOS} GOARCH=${GOARCH} go build \
        -ldflags "${LDFLAGS}" \
        -o "${OUTPUT_NAME}" .
done

echo "✨ All builds completed successfully! Artifacts are available in the '${DIST_DIR}/' directory."
