#!/usr/bin/env sh
set -eu

repo="${MR_QUEUE_REPO:-TYY/mr-queue}"
version="${MR_QUEUE_VERSION:-latest}"
install_dir="${INSTALL_DIR:-/usr/local/bin}"

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"

case "$os" in
  linux|darwin) ;;
  *) echo "unsupported OS: $os" >&2; exit 1 ;;
esac

case "$arch" in
  x86_64|amd64) arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *) echo "unsupported arch: $arch" >&2; exit 1 ;;
esac

asset="mr-queue_${os}_${arch}.tar.gz"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT INT TERM

if [ "$version" = "latest" ]; then
  base_url="https://github.com/${repo}/releases/latest/download"
else
  base_url="https://github.com/${repo}/releases/download/${version}"
fi

echo "Downloading ${asset} from ${repo}..."
curl -fsSL "${base_url}/${asset}" -o "${tmp_dir}/${asset}"

if curl -fsSL "${base_url}/checksums.txt" -o "${tmp_dir}/checksums.txt"; then
  if command -v shasum >/dev/null 2>&1; then
    (cd "$tmp_dir" && grep " ${asset}$" checksums.txt | shasum -a 256 -c -)
  fi
fi

tar -xzf "${tmp_dir}/${asset}" -C "$tmp_dir"

if [ -w "$install_dir" ]; then
  install -m 0755 "${tmp_dir}/mr-queue_${os}_${arch}/mr-queue" "${install_dir}/mr-queue"
else
  sudo install -m 0755 "${tmp_dir}/mr-queue_${os}_${arch}/mr-queue" "${install_dir}/mr-queue"
fi

echo "Installed: ${install_dir}/mr-queue"
"${install_dir}/mr-queue" version
