#!/bin/sh
# Builds every release file into dist/. Expects web/dist to be built already.
#
# The names are what the updater looks for: <name>_<os>_<arch>, with .exe for
# Windows (a plain file people can run straight away) and .tar.gz elsewhere,
# because a plain download loses the executable bit on macOS and Linux.
set -eu

version="$1"
name=dgg-obs-tools
rm -rf dist
mkdir -p dist

for target in windows/amd64 windows/arm64 darwin/amd64 darwin/arm64 linux/amd64 linux/arm64; do
  os=${target%/*}
  arch=${target#*/}
  if [ "$os" = windows ]; then
    output="dist/${name}_${os}_${arch}.exe"
  else
    output="dist/$name"
  fi
  CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -trimpath \
    -ldflags "-s -w -X main.version=$version" -o "$output" .
  if [ "$os" != windows ]; then
    tar -czf "dist/${name}_${os}_${arch}.tar.gz" -C dist "$name"
    rm "dist/$name"
  fi
done

(cd dist && sha256sum -- * > checksums.txt)
