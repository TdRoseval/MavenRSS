#!/bin/bash
set -e

PLATFORM=$1
ARCH=${2:-amd64}
APP_NAME=${APP_NAME:-MavenRSS}

echo "Building for $PLATFORM/$ARCH..."

# Setup Zig as CC for cross-compilation
export CC="zig cc -target"
export CXX="zig c++ -target"

case "$PLATFORM" in
  windows)
    if [ "$ARCH" = "amd64" ]; then
      export GOOS=windows
      export GOARCH=amd64
      export CC="$CC x86_64-windows-gnu"
      export CXX="$CXX x86_64-windows-gnu"
    elif [ "$ARCH" = "arm64" ]; then
      export GOOS=windows
      export GOARCH=arm64
      export CC="$CC aarch64-windows-gnu"
      export CXX="$CXX aarch64-windows-gnu"
    fi
    export CGO_ENABLED=1
    go build -tags production -trimpath -buildvcs=false -ldflags="-w -s -H windowsgui" -o bin/${APP_NAME}.exe .
    ;;

  linux)
    if [ "$ARCH" = "amd64" ]; then
      export GOOS=linux
      export GOARCH=amd64
      export CC="$CC x86_64-linux-gnu"
      export CXX="$CXX x86_64-linux-gnu"
    elif [ "$ARCH" = "arm64" ]; then
      export GOOS=linux
      export GOARCH=arm64
      export CC="$CC aarch64-linux-gnu"
      export CXX="$CXX aarch64-linux-gnu"
    fi
    export CGO_ENABLED=1
    export PKG_CONFIG_PATH=/usr/lib/x86_64-linux-gnu/pkgconfig
    go build -tags production -trimpath -buildvcs=false -ldflags="-w -s" -o bin/${APP_NAME}-${PLATFORM}-${ARCH} .
    ;;

  darwin)
    # Note: macOS cross-compilation is complex, better to build natively on macOS
    echo "macOS builds should be done on native macOS runners"
    exit 1
    ;;

  *)
    echo "Unknown platform: $PLATFORM"
    exit 1
    ;;
esac

echo "Build complete!"
