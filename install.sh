#!/bin/sh
set -e

REPO="KrishnaSSH/GopherTube"
BIN_NAME="gophertube"
DIR="${DIR:-$HOME/.local/bin}"
OUT="$DIR/$BIN_NAME"
VERSION_FILE="$DIR/${BIN_NAME}.version"

mkdir -p "$DIR"

OS=$(uname | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
  x86_64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  armv7l) ARCH="arm" ;;
  i386|i686) ARCH="386" ;;
  *)
    echo "unsupported architecture: $ARCH"
    exit 1
    ;;
esac

case "$OS" in
  linux) OS="linux" ;;
  darwin) OS="darwin" ;;
  *)
    echo "unsupported OS: $OS"
    exit 1
    ;;
esac

echo "Fetching latest release..."

API="https://api.github.com/repos/$REPO/releases/latest"
JSON=$(curl -fsSL "$API")

VERSION=$(echo "$JSON" | grep '"tag_name"' | head -n1 | cut -d '"' -f4)

echo "latest: $VERSION"

CURRENT=""
[ -f "$VERSION_FILE" ] && CURRENT=$(cat "$VERSION_FILE")

if [ "$VERSION" = "$CURRENT" ] && [ -f "$OUT" ]; then
  echo "already up to date"
  exec "$OUT"
fi

# matches YOUR CI naming format
ASSET="gophertube-${OS}-${ARCH}-${VERSION}"
BASE="https://github.com/$REPO/releases/download/$VERSION"

TMP="$OUT.tmp"

echo "downloading $ASSET"

curl -L --fail -o "$TMP" "$BASE/$ASSET"

chmod +x "$TMP"
mv "$TMP" "$OUT"

echo "$VERSION" > "$VERSION_FILE"

echo "installed -> $OUT"
exec "$OUT"
