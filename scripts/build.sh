#!/bin/bash

# Build script for TPWFC Worker

set -e

echo "Building TPWFC Worker..."

# Build crawler
echo "Building crawler..."
go build -o bin/crawler ./cmd/crawler

# Build normalizer
echo "Building normalizer..."
go build -o bin/normalizer ./cmd/normalizer

# Build worker
echo "Building worker..."
go build -o bin/worker ./cmd/worker

# Build uploader
echo "Building uploader..."
go build -o bin/uploader ./cmd/uploader

# Build formatter
echo "Building formatter..."
go build -o bin/formatter ./cmd/formatter

echo "Build complete!"
echo "Executables available in ./bin/"