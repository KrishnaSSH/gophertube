#!/bin/sh
set -e

repo="KrishnaSSH/GopherTube"
bin_name="gophertube"

dir="/usr/local/bin"
out="$dir/$bin_name"
version_file="$dir/${bin_name}.version"

tmp="/tmp/$bin_name"

os=$(uname | tr '[:upper:]' '[:lower:]')
arch=$(uname -m)

case "$arch" in
  x86_64) arch="amd64" ;;
  aarch64|arm64) arch="arm64" ;;
  armv7l) arch="arm" ;;
  i386|i686) arch="386" ;;
  *)
    echo "unsupported architecture: $arch"
    exit 1
    ;;
esac

case "$os" in
  linux) os="linux" ;;
  darwin) os="darwin" ;;
  *)
    echo "unsupported os: $os"
    exit 1
    ;;
esac

echo "fetching latest release..."

api="https://api.github.com/repos/$repo/releases/latest"
json=$(curl -fsSL "$api")

version=$(echo "$json" | grep '"tag_name"' | head -n1 | cut -d '"' -f4)

echo "latest version: $version"

asset="gophertube-${os}-${arch}-${version}"
base="https://github.com/$repo/releases/download/$version"

echo "downloading: $asset"

curl -L --fail -o "$tmp" "$base/$asset"

chmod +x "$tmp"

echo "installing binary to $dir (requires sudo)..."

sudo mv "$tmp" "$out"
sudo chmod +x "$out"

echo "$version" | sudo tee "$version_file" >/dev/null

echo "installed -> $out"

man_src="man/gophertube.1"
man_dest="/usr/share/man/man1/gophertube.1"

echo "installing man page..."

if [ -f "$man_src" ]; then
  sudo install -Dm644 "$man_src" "$man_dest"
  sudo gzip -f "$man_dest"
  sudo mandb >/dev/null 2>&1 || true
  echo "man page installed -> /usr/share/man/man1/gophertube.1.gz"
else
  echo "man page not found, skipping"
fi

echo "done"
echo "run: gophertube to get started"