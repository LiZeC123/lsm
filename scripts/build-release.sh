#!/usr/bin/env bash
#
# Builds the release archives into dist/.
#
# lsm ships as one self-contained binary per platform (dev.md §1.1), so this
# produces lsm_<version>_<os>_<arch>.tar.gz for Unix and .zip for Windows, each
# containing the binary, the README and the LICENSE, plus a SHA256SUMS file.
#
# Usage:
#   scripts/build-release.sh [version] [output-dir]
#
#   version     defaults to `git describe --tags --always --dirty`
#   output-dir  defaults to dist/
#
# The binary is built with CGO_ENABLED=0 so it stays static and dependency free.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

VERSION="${1:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}"
COMMIT="$(git rev-parse --short HEAD 2>/dev/null || true)"
DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
OUT="${2:-dist}"

# A tag never contains a slash, but a branch-derived version can; keep the file
# names safe without altering the stamp that goes into the binary.
SAFE_VERSION="${VERSION//\//-}"

MODULE="github.com/LiZeC123/lsm"
LDFLAGS="-s -w"
LDFLAGS="$LDFLAGS -X ${MODULE}/internal/version.Version=${VERSION}"
if [ -n "$COMMIT" ]; then
	LDFLAGS="$LDFLAGS -X ${MODULE}/internal/version.Commit=${COMMIT}"
fi
LDFLAGS="$LDFLAGS -X ${MODULE}/internal/version.Date=${DATE}"

TARGETS="linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64"

rm -rf "$OUT"
mkdir -p "$OUT"
# Resolve to an absolute path so the archive commands work no matter which
# directory they run from.
OUT="$(cd "$OUT" && pwd)"

for target in $TARGETS; do
	goos="${target%%/*}"
	goarch="${target##*/}"
	name="lsm_${SAFE_VERSION}_${goos}_${goarch}"
	binary="lsm"
	if [ "$goos" = "windows" ]; then
		binary="lsm.exe"
	fi

	echo "==> ${goos}/${goarch}"
	stage="$(mktemp -d)"
	pkg="${stage}/${name}"
	mkdir -p "$pkg"

	GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 \
		go build -trimpath -ldflags "$LDFLAGS" -o "${pkg}/${binary}" .

	cp README.md LICENSE "$pkg/"

	if [ "$goos" = "windows" ]; then
		(cd "$stage" && zip -q -r "${OUT}/${name}.zip" "$name")
	else
		tar -czf "${OUT}/${name}.tar.gz" -C "$stage" "$name"
	fi

	rm -rf "$stage"
done

# Refuse to publish a partial release: every target must have produced exactly
# one archive.
expected="$(echo "$TARGETS" | wc -w | tr -d ' ')"
actual="$(find "$OUT" -maxdepth 1 -type f -name 'lsm_*' | wc -l | tr -d ' ')"
if [ "$actual" -ne "$expected" ]; then
	echo "expected ${expected} archives, found ${actual}" >&2
	exit 1
fi

# shasum and sha256sum emit the same "<hash>  <name>" format.
if command -v sha256sum >/dev/null 2>&1; then
	(cd "$OUT" && sha256sum lsm_* > SHA256SUMS)
else
	(cd "$OUT" && shasum -a 256 lsm_* > SHA256SUMS)
fi

echo
echo "wrote to ${OUT}/:"
ls -1 "$OUT"
