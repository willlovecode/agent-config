#!/usr/bin/env bash
# Cross-builds agtk for linux, darwin and windows (amd64, arm64) into dist/release/:
#   agtk_<version>_<os>_<arch>.tar.gz (.zip for windows) and SHA256SUMS.
# Needs Go (the version in go.mod), git and sha256sum or shasum. Runs on Linux, macOS and Git Bash.
#
#   scripts/release.sh [version]     version defaults to `git describe --tags --always --dirty`
#
# A leading "v" is dropped: tag v1.2.3 gives version 1.2.3, file names agtk_1.2.3_linux_amd64.tar.gz.
set -euo pipefail

cd "$(dirname "$0")/.."
root=$PWD

version=${1:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}
version=${version#v}
case $version in
  *[!A-Za-z0-9._+-]* | '') echo "release.sh: bad version '$version'" >&2; exit 2 ;;
esac

# Same bytes on every run: entries carry the commit time, or a fixed date outside a git checkout.
mtime=${SOURCE_DATE_EPOCH:-$(git log -1 --format=%ct 2>/dev/null || echo 1704067200)}

if command -v sha256sum >/dev/null 2>&1; then
  sha256() { sha256sum "$@"; }
else
  sha256() { shasum -a 256 "$@"; }
fi

out=$root/dist/release
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
rm -rf "$out"
mkdir -p "$out"

license=
for f in LICENSE LICENSE.md LICENSE.txt; do
  if [ -f "$f" ]; then license=$f; break; fi
done

echo "agtk $version (archive time $(date -u -d "@$mtime" +%FT%TZ 2>/dev/null || echo "$mtime"))"
go build -o "$work/mkarchive" ./scripts/mkarchive

for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64; do
  goos=${target%/*}
  goarch=${target#*/}
  bin=agtk
  ext=tar.gz
  if [ "$goos" = windows ]; then bin=agtk.exe; ext=zip; fi
  built=$work/$goos-$goarch-$bin
  GOOS=$goos GOARCH=$goarch CGO_ENABLED=0 \
    go build -trimpath -ldflags "-s -w -X main.version=$version" -o "$built" ./cmd/agtk
  files=("$bin=$built")
  if [ -n "$license" ]; then files+=("$(basename "$license")=$license"); fi
  "$work/mkarchive" -o "$out/agtk_${version}_${goos}_${goarch}.$ext" -mtime "$mtime" "${files[@]}"
done

# Git Bash's sha256sum marks files "binary" with a leading *; plain two-space lines work everywhere.
(cd "$out" && LC_ALL=C sha256 agtk_* | sed 's/ \*/  /' | LC_ALL=C sort -k2 >SHA256SUMS)

printf '\n%-40s %10s  %s\n' file bytes sha256
for f in "$out"/agtk_*; do
  name=$(basename "$f")
  size=$(wc -c <"$f" | tr -d ' ')
  sum=$(grep " $name\$" "$out/SHA256SUMS" | cut -c1-12)
  printf '%-40s %10s  %s...\n' "$name" "$size" "$sum"
done
printf '%-40s %10s\n' SHA256SUMS "$(wc -c <"$out/SHA256SUMS" | tr -d ' ')"
echo
echo "written to $out"
