#!/usr/bin/env bash
# Symlink claude-switch into ~/.local/bin (or $1).
set -euo pipefail

src="$(cd "$(dirname "$0")" && pwd)/bin/claude-switch"
dest="${1:-$HOME/.local/bin}"

command -v jq >/dev/null || { echo "jq is required: brew install jq" >&2; exit 1; }
mkdir -p "$dest"
ln -sf "$src" "$dest/claude-switch"
echo "installed $dest/claude-switch"
case ":$PATH:" in *":$dest:"*) ;; *) echo "add $dest to your PATH" ;; esac
