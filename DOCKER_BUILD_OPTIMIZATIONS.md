# Docker Build Optimizations

## Problem
The Docker build was taking 10-20 minutes and getting stuck at:
```
[linux/arm64 builder 7/7] RUN CGO_ENABLED=0 go build -ldflags="-w -s" -o engine .
```

## Root Causes
1. **Heavy Dependencies**: Project has many large dependencies (AWS SDK, Google Cloud, Firebase, etc.)
2. **Inefficient Layer Caching**: No optimization for Go module downloads and build cache
3. **Cross-compilation Overhead**: Building ARM64 on GitHub's AMD64 runners without optimization
4. **Large Build Context**: Unnecessary files being sent to Docker daemon
5. **No Build Cache**: Missing Go build cache and module cache optimizations

## Optimizations Applied

### 1. Enhanced Dockerfile (`Dockerfile`)

#### Build Stage Optimizations
- **Multi-platform build support**: `FROM --platform=$BUILDPLATFORM golang:1.23-alpine`
- **Go proxy optimization**: `ENV GOPROXY=https://proxy.golang.org,direct`
- **Cache directories**: Defined `GOCACHE` and `GOMODCACHE` paths
- **Mount caches**: Used BuildKit cache mounts for `/go/mod` and `/go/cache`
- **Module verification**: Added `go mod verify` for integrity
- **Better layer caching**: Separated mod download from source copy

#### Build Command Optimizations
- **Cross-compilation args**: `ARG TARGETOS` and `ARG TARGETARCH`
- **Trimpath**: Removes absolute paths from binary
- **Static linking**: `-extldflags '-static'` and `-tags="netgo,osusergo"`
- **Optimized ldflags**: `-w -s` for smaller binaries

#### Runtime Stage Optimizations
- **Minimal base**: Alpine 3.19 with only necessary packages
- **Package cleanup**: `rm -rf /var/cache/apk/*`
- **Proper permissions**: `chmod +x ./engine`
- **Health check**: Process-based health monitoring

### 2. GitHub Actions Workflow (`.github/workflows/release.yml`)

#### Docker Job Optimizations
- **Build timeout**: `timeout-minutes: 30` to prevent infinite hangs
- **BuildKit optimizations**: Network host mode for faster builds
- **GitHub Actions Cache**: `cache-from: type=gha` and `cache-to: type=gha,mode=max`
- **Inline cache**: `BUILDKIT_INLINE_CACHE=1`
- **Disabled unnecessary features**: `provenance: false` and `sbom: false`

### 3. Enhanced .dockerignore (`.dockerignore`)

#### Excluded Categories
- **Development files**: Test files, IDE configs, local env files
- **Build artifacts**: Previous builds, coverage reports, logs
- **Documentation**: README files, changelogs (not needed in container)
- **Go-specific**: Test files, vendor directory, coverage files
- **OS files**: .DS_Store, Thumbs.db, temp files
- **Cache directories**: Local build caches, node_modules

### 4. Local Build Script (`docker-build.sh`)

#### Features
- **BuildKit setup**: Automatic buildx configuration
- **Local caching**: Uses local cache for faster rebuilds
- **Build timing**: Measures and reports build duration
- **Platform testing**: Single platform builds for local testing
- **User guidance**: Clear instructions and status reporting

## Expected Performance Improvements

### Before Optimizations
- ❌ **Build Time**: 10-20 minutes
- ❌ **Cache Miss**: Every build downloads all dependencies
- ❌ **Large Context**: Unnecessary files slow transfer
- ❌ **Cross-compilation**: Inefficient ARM64 builds

### After Optimizations
- ✅ **Build Time**: 2-5 minutes (first build), 30s-2min (cached builds)
- ✅ **Cache Hit**: Go modules and build cache preserved
- ✅ **Small Context**: Only necessary files transferred
- ✅ **Efficient Cross-compilation**: Optimized with proper build args

## Usage

### Local Testing
```bash
# Make script executable (already done)
chmod +x docker-build.sh

# Run optimized build
./docker-build.sh
```

### CI/CD (Automatic)
The GitHub Actions workflow will automatically use all optimizations when you push a tag.

### Manual Docker Build
```bash
# With BuildKit cache
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  --cache-from type=gha \
  --cache-to type=gha,mode=max \
  -t apito-engine:latest \
  .
```

## Monitoring Build Performance

### GitHub Actions
- Check the Docker job duration in Actions tab
- Look for cache hit messages in build logs
- Monitor the "Build and push Docker image" step time

### Local Builds
- Use `./docker-build.sh` which shows build duration
- Watch for "CACHED" messages in Docker output
- Monitor `docker system df` for cache usage

## Troubleshooting

### If builds are still slow:
1. **Check cache hits**: Look for "CACHED" in build output
2. **Verify BuildKit**: Ensure BuildKit is enabled
3. **Platform-specific**: Try building single platform first
4. **Dependencies**: Consider reducing heavy dependencies if possible

### If builds fail:
1. **Cache corruption**: Clear with `docker buildx prune`
2. **Platform issues**: Build for single platform first
3. **Dependency conflicts**: Check `go mod tidy`

## Technical Details

### Cache Strategy
- **Module cache**: Persistent across builds for faster `go mod download`
- **Build cache**: Preserves compiled packages and dependencies
- **Layer cache**: Docker layers cached for unchanged steps
- **GitHub Actions cache**: Cross-workflow cache persistence

### Security Considerations
- **Static binary**: No dynamic dependencies in final image
- **Non-root user**: Container runs as `apito:1000`
- **Minimal surface**: Only necessary packages in runtime image
- **Health checks**: Automated container health monitoring

This optimization should reduce your Docker build time from 10-20 minutes to 2-5 minutes for fresh builds and 30 seconds to 2 minutes for cached builds. 