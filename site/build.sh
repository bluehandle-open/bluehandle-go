#!/usr/bin/env bash
# Assemble the Cloudflare Pages download site in public/ from a GitHub
# Release: copies the page template, downloads the release binaries and
# fills in version, date, sizes and SHA-256 digests.
#
#   bash site/build.sh            # latest release
#   bash site/build.sh v1.0.0     # a specific tag
#
# Needs gh (authenticated, or GH_TOKEN set), sha256sum and stat.
set -euo pipefail

tag="${1:-}"
repo="${GITHUB_REPOSITORY:-bluehandle-open/bluehandle-go}"
root="$(cd "$(dirname "$0")/.." && pwd)"
out="$root/public"

if [ -z "$tag" ]; then
  tag="$(gh release view -R "$repo" --json tagName -q .tagName)"
fi
published="$(gh release view "$tag" -R "$repo" --json publishedAt -q .publishedAt)"
release_url="$(gh release view "$tag" -R "$repo" --json url -q .url)"
version="${tag#v}"
date="${published%%T*}"

rm -rf "$out"
mkdir -p "$out/download"
cp "$root/site/index.html" "$root/site/logo.svg" "$root/site/_headers" "$out/"
gh release download "$tag" -R "$repo" -D "$out/download" \
  -p bluehandle.exe -p bluehandle-cli.exe

size() { stat -c %s "$1"; }
human() { awk -v b="$1" 'BEGIN { printf "%.1f MB", b / 1048576 }'; }
digest() { sha256sum "$1" | cut -d' ' -f1; }

exe="$out/download/bluehandle.exe"
cli="$out/download/bluehandle-cli.exe"
exe_size="$(size "$exe")"; exe_sha="$(digest "$exe")"
cli_size="$(size "$cli")"; cli_sha="$(digest "$cli")"

sed -i \
  -e "s|@VERSION@|$version|g" \
  -e "s|@TAG@|$tag|g" \
  -e "s|@DATE@|$date|g" \
  -e "s|@RELEASE_URL@|$release_url|g" \
  -e "s|@EXE_SIZE@|$(human "$exe_size")|g" \
  -e "s|@EXE_SHA256@|$exe_sha|g" \
  -e "s|@CLI_SIZE@|$(human "$cli_size")|g" \
  -e "s|@CLI_SHA256@|$cli_sha|g" \
  "$out/index.html"

# Machine-readable version info, e.g. for the main site or an update check.
cat > "$out/download/latest.json" <<EOF
{
  "version": "$version",
  "tag": "$tag",
  "published_at": "$published",
  "release_url": "$release_url",
  "files": {
    "bluehandle.exe": { "url": "/download/bluehandle.exe", "size": $exe_size, "sha256": "$exe_sha" },
    "bluehandle-cli.exe": { "url": "/download/bluehandle-cli.exe", "size": $cli_size, "sha256": "$cli_sha" }
  }
}
EOF

if grep -q '@[A-Z0-9_]*@' "$out/index.html"; then
  echo "unfilled placeholders in index.html:" >&2
  grep -o '@[A-Z0-9_]*@' "$out/index.html" | sort -u >&2
  exit 1
fi
echo "built $tag into $out"
