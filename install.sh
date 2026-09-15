#!/usr/bin/env bash
# mano 安装脚本
# 用法: curl -fsSL https://raw.githubusercontent.com/ParadiseWitch/mano/main/install.sh | bash
#       指定版本: MANO_VERSION=v1.2.3 bash install.sh
set -euo pipefail

repo='ParadiseWitch/mano'
version="${MANO_VERSION:-latest}"

os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os" in
  linux)  ;;
  darwin) ;;
  *) echo "mano: 不支持的系统 $os，Windows 请用 install.ps1" >&2; exit 1 ;;
esac

arch=$(uname -m)
case "$arch" in
  x86_64 | amd64)  arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *) echo "mano: 不支持的架构 $arch" >&2; exit 1 ;;
esac

asset="mano-${os}-${arch}"
base="https://github.com/${repo}/releases"
if [ "$version" = latest ]; then
  base="$base/latest/download"
else
  base="$base/download/$version"
fi

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

echo "==> 下载 $asset ($version)"
curl -fsSL --proto '=https' --retry 3 -o "$tmp/$asset" "$base/$asset"
curl -fsSL --proto '=https' --retry 3 -o "$tmp/checksums.txt" "$base/checksums.txt"

expected=$(awk -v a="$asset" '$2 == a {print $1; exit}' "$tmp/checksums.txt")
if [ -z "$expected" ]; then
  echo "mano: checksums.txt 中没有 $asset" >&2; exit 1
fi
if command -v sha256sum >/dev/null; then
  actual=$(sha256sum "$tmp/$asset" | cut -d' ' -f1)
elif command -v shasum >/dev/null; then
  actual=$(shasum -a 256 "$tmp/$asset" | cut -d' ' -f1)
else
  echo "mano: 找不到 sha256 工具，无法校验" >&2; exit 1
fi
if [ "$actual" != "$expected" ]; then
  echo "mano: 校验失败，中止安装" >&2; exit 1
fi
echo "==> sha256 校验通过"

if [ -d /usr/local/bin ] && [ -w /usr/local/bin ]; then
  dest="/usr/local/bin/mano"
else
  mkdir -p "$HOME/.local/bin"
  dest="$HOME/.local/bin/mano"
fi

mv "$tmp/$asset" "$dest"
chmod +x "$dest"
echo "==> 已安装到 $dest"

case ":$PATH:" in
  *":$(dirname "$dest"):*") ;;
  *) echo "请将 $(dirname "$dest") 加入 PATH" ;;
esac

"$dest" --version 2>/dev/null || "$dest" version 2>/dev/null || true
