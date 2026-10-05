#!/usr/bin/env bash
# End-to-end test in a throwaway HOME with a fake Keychain.
# Never touches the real Keychain or ~/.claude.json.
set -u

BIN="$(cd "$(dirname "$0")/.." && pwd)/bin/claude-switch"
S=$(mktemp -d)
trap 'rm -rf "$S"' EXIT
mkdir -p "$S/home/.claude" "$S/stub" "$S/kc"

cat >"$S/stub/security" <<'EOF'
#!/usr/bin/env bash
cmd=$1; shift; s=; a=; w=
while [ $# -gt 0 ]; do
  case $1 in
    -s) s=$2; shift ;;
    -a) a=$2; shift ;;
    -w) if [ $# -gt 1 ] && [ "${2:0:1}" != - ]; then w=$2; shift; fi ;;
  esac
  shift
done
f="$KCDIR/$(printf '%s__%s' "$s" "$a" | tr ' /' '__')"
case $cmd in
  find-generic-password) [ -f "$f" ] || exit 44; cat "$f" ;;
  add-generic-password) printf '%s' "$w" >"$f" ;;
  delete-generic-password) rm -f "$f" ;;
esac
EOF
printf '#!/bin/sh\necho "$*" >>"$KCDIR/notify"\n' >"$S/stub/osascript"
printf '#!/bin/sh\nprintf "%%s" "$*" >"$KCDIR/login-args"\n' >"$S/stub/claude"
chmod +x "$S/stub/"*

export KCDIR="$S/kc" HOME="$S/home" PATH="$S/stub:$PATH" USER=tester
LIVE="$KCDIR/Claude_Code-credentials__tester"
FUT=$(($(date +%s) + 3600))
fails=0

ok() { if [ "$2" = "$3" ]; then echo "ok   $1"; else echo "FAIL $1: got '$2' want '$3'"; fails=$((fails + 1)); fi; }

login() { # email token
  printf '{"claudeAiOauth":{"accessToken":"%s","refreshTokenExpiresAt":9999999999999},"mcpOAuth":{"x":"keep"}}' "$2" >"$LIVE"
  jq -n --arg e "$1" '{numStartups:3, oauthAccount:{emailAddress:$e}}' >"$HOME/.claude.json"
}

statusline() { # session five seven
  jq -nc --arg s "$1" --argjson f "$2" --argjson w "$3" --argjson r "$FUT" \
    '{session_id:$s, rate_limits:{five_hour:{used_percentage:$f,resets_at:$r}, seven_day:{used_percentage:$w,resets_at:($r+86400)}}}'
}

live_token() { jq -r .claudeAiOauth.accessToken "$LIVE"; }

login a@x tokA; "$BIN" add a >/dev/null
login b@x tokB; "$BIN" add b >/dev/null
ok "add keeps last added active" "$("$BIN" current)" b

login b@x tokB2 # Claude Code rotated b's token
"$BIN" use a >/dev/null
ok "use swaps live token" "$(live_token)" tokA
ok "use swaps oauthAccount" "$(jq -r .oauthAccount.emailAddress "$HOME/.claude.json")" a@x
ok "use keeps other .claude.json keys" "$(jq -r .numStartups "$HOME/.claude.json")" 3
ok "use keeps mcpOAuth" "$(jq -r .mcpOAuth.x "$LIVE")" keep
ok "outgoing account saved with rotated token" \
  "$(jq -r .login.claudeAiOauth.accessToken "$KCDIR/claude-switch__b")" tokB2

login c@x tokC
"$BIN" use b >/dev/null 2>&1
ok "refuses to overwrite an unsaved manual login" "$(live_token)" tokC
login a@x tokA

statusline s1 40 10 | "$BIN" report
ok "under threshold stays" "$("$BIN" current)" a
statusline s1 96 10 | "$BIN" report >/dev/null
ok "over threshold switches" "$("$BIN" current)" b
ok "switch put b token live" "$(live_token)" tokB2
statusline s1 96 10 | "$BIN" report >/dev/null
ok "stale numbers from same session ignored" "$("$BIN" current)" b
statusline s2 97 20 | "$BIN" report >/dev/null 2>&1
ok "all exhausted stays put" "$("$BIN" current)" b
ok "all exhausted notifies" "$(grep -c 'every account' "$KCDIR/notify")" 1

# Expire a's refresh token and clear usage.
jq '.accounts.a.rt_exp = 1000 | .usage = {}' "$HOME/.claude-switch/state.json" >"$S/t" && mv "$S/t" "$HOME/.claude-switch/state.json"
"$BIN" next auto >/dev/null 2>&1
ok "auto skips dead account" "$("$BIN" current)" b
ok "auto notifies dead account" "$(grep -c 'claude-switch login a' "$KCDIR/notify")" 1
"$BIN" use a >/dev/null 2>&1
ok "manual use of dead account starts login with email" "$(cat "$KCDIR/login-args")" "auth login --email a@x"
"$BIN" list >"$S/list" 2>&1
ok "list renders" "$?" 0
cat "$S/list"

echo
[ "$fails" = 0 ] && echo "all passed" || { echo "$fails failed"; exit 1; }
