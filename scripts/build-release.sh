#!/bin/sh
# Builds every release file into dist/. Expects web/dist to be built already.
#
# The names are what the updater looks for: <name>_<os>_<arch>, spelled the way
# people know them (macos, x64; see AssetOS and AssetArch in
# internal/update/update.go). Windows gets a plain .exe people can run straight
# away; elsewhere a .tar.gz, because a plain download loses the executable bit.
set -eu

version="$1"
name=dgg-obs-tools
rm -rf dist
mkdir -p dist

for target in windows/amd64 windows/arm64 darwin/amd64 darwin/arm64 linux/amd64 linux/arm64; do
  goos=${target%/*}
  goarch=${target#*/}
  os=$goos
  arch=$goarch
  [ "$goos" = darwin ] && os=macos
  [ "$goarch" = amd64 ] && arch=x64
  if [ "$os" = windows ]; then
    output="dist/${name}_${os}_${arch}.exe"
  else
    output="dist/$name"
  fi
  CGO_ENABLED=0 GOOS=$goos GOARCH=$goarch go build -trimpath \
    -ldflags "-s -w -X main.version=$version" -o "$output" .
  if [ "$os" != windows ]; then
    tar -czf "dist/${name}_${os}_${arch}.tar.gz" -C dist "$name"
    rm "dist/$name"
  fi
done

(cd dist && sha256sum -- * > checksums.txt)
