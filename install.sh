#!/usr/bin/env bash
# Install claude-switch and hook it into the Claude Code status line.
#
#   curl -fsSL https://raw.githubusercontent.com/PuntoyComaTech/claude-switch/main/install.sh | bash
#   ./install.sh                  from a clone (symlinks, so `git pull` updates it)
#   ./install.sh --no-statusline  skip the status line step
#   ./install.sh --uninstall      remove the command, saved logins and state
set -euo pipefail

RAW="https://raw.githubusercontent.com/PuntoyComaTech/claude-switch/main"
BIN_DIR="${CLAUDE_SWITCH_BIN_DIR:-$HOME/.local/bin}"
DIR="${CLAUDE_SWITCH_DIR:-$HOME/.claude-switch}"
SETTINGS="$HOME/.claude/settings.json"
STATUSLINE="$DIR/statusline.sh"

say() { printf '%s\n' "$*"; }
die() { printf 'install: %s\n' "$*" >&2; exit 1; }

# A clone has bin/claude-switch next to this script. Under `curl | bash` it does not.
here=""
src="${BASH_SOURCE[0]:-}"
if [ -n "$src" ] && [ -f "$src" ]; then
  here=$(cd "$(dirname "$src")" && pwd)
  [ -f "$here/bin/claude-switch" ] || here=""
fi

# get <repo path> <dest>: symlink from a clone, download otherwise.
get() {
  if [ -n "$here" ]; then
    ln -sf "$here/$1" "$2"
  else
    curl -fsSL "$RAW/$1" -o "$2.tmp" && mv "$2.tmp" "$2"
  fi
}

uninstall() {
  local name
  if [ -f "$DIR/state.json" ]; then
    for name in $(jq -r '.order[]' "$DIR/state.json"); do
      security delete-generic-password -s claude-switch -a "$name" >/dev/null 2>&1 || true
    done
  fi
  if [ -f "$SETTINGS" ] && [ "$(jq -r '.statusLine.command // ""' "$SETTINGS")" = "bash $STATUSLINE" ]; then
    tmp=$(mktemp)
    jq 'del(.statusLine)' "$SETTINGS" >"$tmp" && mv "$tmp" "$SETTINGS"
    say "removed statusLine from $SETTINGS"
  fi
  rm -f "$BIN_DIR/claude-switch"
  rm -rf "$DIR"
  say "claude-switch removed. Your current Claude Code login was not touched."
  say "If you added the report line to your own status line script, delete it."
}

setup_statusline() {
  local cmd file
  mkdir -p "$(dirname "$SETTINGS")"
  [ -f "$SETTINGS" ] || echo '{}' >"$SETTINGS"
  cmd=$(jq -r '.statusLine.command // ""' "$SETTINGS")

  if [ -z "$cmd" ]; then
    get statusline-example.sh "$STATUSLINE"
    cp "$SETTINGS" "$SETTINGS.bak.claude-switch"
    tmp=$(mktemp)
    jq --arg c "bash $STATUSLINE" '.statusLine = {type: "command", command: $c, refreshInterval: 120}' "$SETTINGS" >"$tmp" \
      && mv "$tmp" "$SETTINGS"
    say "status line: installed (backup at $SETTINGS.bak.claude-switch)"
    return
  fi

  # Existing status line: only report whether it already calls claude-switch.
  file=${cmd##* }
  file=${file/#\~/$HOME}
  if [[ "$cmd" == *claude-switch* ]] || { [ -f "$file" ] && grep -q 'claude-switch report' "$file"; }; then
    say "status line: already hooked"
    return
  fi
  say "status line: you already have one ($cmd)."
  say "Add this line to it, right after it reads stdin into \$input:"
  say ""
  say "  command -v claude-switch >/dev/null && { printf '%s' \"\$input\" | claude-switch report >/dev/null 2>&1 & }"
  say ""
}

main() {
  [ "$(uname)" = Darwin ] || die "macOS only"
  command -v jq >/dev/null || die "jq is required: brew install jq"
  case "${1:-}" in
    --uninstall) uninstall; return ;;
  esac
  command -v claude >/dev/null || say "warning: 'claude' not found on PATH"

  mkdir -p "$BIN_DIR" "$DIR"
  chmod 700 "$DIR"
  get bin/claude-switch "$BIN_DIR/claude-switch"
  chmod +x "$BIN_DIR/claude-switch"
  say "installed $BIN_DIR/claude-switch ($("$BIN_DIR/claude-switch" --version))"
  case ":$PATH:" in *":$BIN_DIR:"*) ;; *) say "warning: add $BIN_DIR to your PATH" ;; esac

  [ "${1:-}" = --no-statusline ] || setup_statusline

  say ""
  say "Next, save your accounts:"
  say "  claude-switch add <name>            # the account you are logged in with now"
  say "  claude-switch add <name> --login    # each other account, via the browser"
  say "  claude-switch list"
}

main "$@"
