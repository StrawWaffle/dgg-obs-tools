#!/bin/sh
# Builds release files into dist/ for the targets given, such as
#
#     scripts/build-release.sh v0.2.0 windows/amd64 linux/amd64
#
# Expects web/dist to be built already. macOS targets need a Mac, because the
# menu bar icon is drawn through Cocoa and that needs cgo; the others build
# anywhere without a C compiler.
#
# The names are what the updater looks for: <name>_<os>_<arch>, spelled the way
# people know them (macos, x64; see AssetOS and AssetArch in
# internal/update/update.go). Windows gets a plain .exe people can run straight
# away; elsewhere a .tar.gz, because a plain download loses the executable bit.
set -eu

version="$1"
shift
name=dgg-obs-tools
mkdir -p dist

for target in "$@"; do
  goos=${target%/*}
  goarch=${target#*/}
  os=$goos
  arch=$goarch
  [ "$goos" = darwin ] && os=macos
  [ "$goarch" = amd64 ] && arch=x64

  ldflags="-s -w -X main.version=$version"
  cgo=0
  cc=
  case "$goos" in
    # No console window: the app lives in the tray.
    windows) ldflags="$ldflags -H=windowsgui" ;;
    darwin)
      cgo=1
      [ "$goarch" = amd64 ] && cc="clang -arch x86_64" || cc="clang -arch arm64"
      ;;
  esac

  if [ "$goos" = windows ]; then
    output="dist/${name}_${os}_${arch}.exe"
  else
    output="dist/$name"
  fi
  CGO_ENABLED=$cgo CC="$cc" GOOS=$goos GOARCH=$goarch \
    go build -trimpath -ldflags "$ldflags" -o "$output" .
  if [ "$goos" != windows ]; then
    tar -czf "dist/${name}_${os}_${arch}.tar.gz" -C dist "$name"
    rm "dist/$name"
  fi
done
