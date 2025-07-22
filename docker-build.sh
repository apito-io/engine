#!/bin/bash

# Docker Build Script for Apito Engine
# This script helps test the optimized Docker build locally

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}🚀 Apito Engine - Optimized Docker Build${NC}"
echo "=================================================="

# Check if Docker is running
if ! docker info > /dev/null 2>&1; then
    echo -e "${RED}❌ Docker is not running. Please start Docker first.${NC}"
    exit 1
fi

# Setup buildx if needed
echo -e "${YELLOW}📦 Setting up Docker Buildx...${NC}"
if ! docker buildx ls | grep -q "mybuilder"; then
    docker buildx create --name mybuilder --use --bootstrap
else
    docker buildx use mybuilder
fi

# Build image with cache
echo -e "${YELLOW}🔨 Building Docker image with optimizations...${NC}"
echo "This build uses:"
echo "  ✅ Multi-stage build with Go cache mounts"
echo "  ✅ BuildKit cache for faster rebuilds"
echo "  ✅ Optimized .dockerignore"
echo "  ✅ Static binary with trimpath"
echo ""

# Start timer
start_time=$(date +%s)

# Build for current platform only (faster for testing)
docker buildx build \
    --platform linux/amd64 \
    --cache-from type=local,src=.docker-cache \
    --cache-to type=local,dest=.docker-cache,mode=max \
    --load \
    -t apito-engine:latest \
    .

# End timer
end_time=$(date +%s)
duration=$((end_time - start_time))

echo ""
echo -e "${GREEN}✅ Build completed in ${duration} seconds!${NC}"
echo ""

# Show image size
echo -e "${YELLOW}📊 Image Information:${NC}"
docker images apito-engine:latest --format "table {{.Repository}}\t{{.Tag}}\t{{.Size}}\t{{.CreatedAt}}"

echo ""
echo -e "${GREEN}🎉 Build successful! You can now run:${NC}"
echo "   docker run -p 5050:5050 apito-engine:latest"
echo ""
echo -e "${YELLOW}💡 To build for multiple platforms (like CI):${NC}"
echo "   docker buildx build --platform linux/amd64,linux/arm64 -t apito-engine:latest ." 