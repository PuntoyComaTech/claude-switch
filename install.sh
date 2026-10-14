#!/bin/sh
# Install claude-switch on macOS or Linux and hook it into the Claude Code status line.
#
#   curl -fsSL https://raw.githubusercontent.com/PuntoyComaTech/claude-switch/main/install.sh | sh
#
# Options (after `sh -s --` when piped):
#   --no-setup     do not touch the Claude Code status line
#   --uninstall    remove the binary (saved accounts stay; see README)
# Env:
#   CLAUDE_SWITCH_BIN_DIR   install dir (default ~/.local/bin)
#   CLAUDE_SWITCH_VERSION   release tag to install (default: latest)
#   CLAUDE_SWITCH_BASE_URL  where release files live (for mirrors and tests)
set -eu

REPO="PuntoyComaTech/claude-switch"
BIN_DIR="${CLAUDE_SWITCH_BIN_DIR:-$HOME/.local/bin}"
VERSION="${CLAUDE_SWITCH_VERSION:-latest}"

say() { printf '%s\n' "$*"; }
die() {
  printf 'install: %s\n' "$*" >&2
  exit 1
}

base_url() {
  if [ -n "${CLAUDE_SWITCH_BASE_URL:-}" ]; then
    printf '%s' "$CLAUDE_SWITCH_BASE_URL"
  elif [ "$VERSION" = latest ]; then
    printf 'https://github.com/%s/releases/latest/download' "$REPO"
  else
    printf 'https://github.com/%s/releases/download/%s' "$REPO" "$VERSION"
  fi
}

detect() {
  case "$(uname -s)" in
    Darwin) os=darwin ;;
    Linux) os=linux ;;
    *) die "unsupported OS $(uname -s). On Windows use install.ps1" ;;
  esac
  case "$(uname -m)" in
    x86_64 | amd64) arch=amd64 ;;
    arm64 | aarch64) arch=arm64 ;;
    *) die "unsupported CPU $(uname -m)" ;;
  esac
}

sha256() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | cut -d' ' -f1
  else
    shasum -a 256 "$1" | cut -d' ' -f1
  fi
}

install_bin() {
  detect
  archive="claude-switch_${os}_${arch}.tar.gz"
  url=$(base_url)
  tmp=$(mktemp -d)
  trap 'rm -rf "$tmp"' EXIT

  say "downloading $archive"
  curl -fsSL "$url/$archive" -o "$tmp/$archive" || die "download failed: $url/$archive"
  curl -fsSL "$url/checksums.txt" -o "$tmp/checksums.txt" || die "download failed: $url/checksums.txt"

  want=$(awk -v f="$archive" '$2 == f { print $1 }' "$tmp/checksums.txt")
  [ -n "$want" ] || die "$archive is not listed in checksums.txt"
  [ "$(sha256 "$tmp/$archive")" = "$want" ] || die "checksum mismatch for $archive"

  tar -xzf "$tmp/$archive" -C "$tmp" claude-switch
  mkdir -p "$BIN_DIR"
  # Replace through a temp name so a running claude-switch is never half-written.
  # rm first: an older install may be a symlink into a git clone.
  cp "$tmp/claude-switch" "$BIN_DIR/.claude-switch.new"
  chmod 755 "$BIN_DIR/.claude-switch.new"
  rm -f "$BIN_DIR/claude-switch"
  mv "$BIN_DIR/.claude-switch.new" "$BIN_DIR/claude-switch"
  say "installed $BIN_DIR/claude-switch ($("$BIN_DIR/claude-switch" --version))"

  case ":$PATH:" in
    *":$BIN_DIR:"*) ;;
    *) say "note: add $BIN_DIR to your PATH, e.g. echo 'export PATH=\"$BIN_DIR:\$PATH\"' >> ~/.profile" ;;
  esac
}

main() {
  setup=1
  for arg in "$@"; do
    case "$arg" in
      --no-setup) setup=0 ;;
      --uninstall)
        rm -f "$BIN_DIR/claude-switch"
        say "removed $BIN_DIR/claude-switch. Your Claude Code login was not touched."
        say "Saved accounts and settings stay; see README > Uninstall to delete them."
        return
        ;;
      *) die "unknown option $arg" ;;
    esac
  done
  command -v curl >/dev/null 2>&1 || die "curl is required"
  install_bin
  command -v claude >/dev/null 2>&1 || say "warning: 'claude' (Claude Code) not found on PATH"
  [ "$setup" = 0 ] || "$BIN_DIR/claude-switch" setup

  say ""
  say "Next, save your accounts:"
  say "  claude-switch add <name>            # the account you are logged in with now"
  say "  claude-switch add <name> --login    # each other account, via the browser"
  say "  claude-switch list"
}

main "$@"
