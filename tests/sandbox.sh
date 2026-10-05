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
key=$(printf '%s' "$s" | tr ' /' '__')
if [ -n "$a" ]; then f="$KCDIR/${key}__$a"; else f=$(ls "$KCDIR/${key}__"* 2>/dev/null | grep -v '\.svc$' | head -1); fi
case $cmd in
  find-generic-password) [ -n "$f" ] && [ -f "$f" ] || exit 44; cat "$f" ;;
  add-generic-password) printf '%s' "$w" >"$f"; printf '%s' "$s" >"$f.svc" ;;
  delete-generic-password) rm -f "$f" "$f.svc" ;;
  dump-keychain) for x in "$KCDIR"/*.svc; do [ -f "$x" ] && printf '    "svce"<blob>="%s"\n' "$(cat "$x")"; done ;;
esac
EOF
printf '#!/bin/sh\necho "$*" >>"$KCDIR/notify"\n' >"$S/stub/osascript"
# Fake browser login: logs in as $PICK. With CLAUDE_CONFIG_DIR it writes to a
# per-dir Keychain item, like Claude Code does.
cat >"$S/stub/claude" <<'EOF'
#!/usr/bin/env bash
if [ "$1 $2" = "auth status" ]; then
  # Like Claude Code: load the login for this config dir and refresh it.
  svc="Claude Code-credentials-$(printf '%s' "$CLAUDE_CONFIG_DIR" | shasum -a 256 | cut -c1-8)"
  cur=$(security find-generic-password -s "$svc" -a "$USER" -w) || { echo '{"loggedIn":false}'; exit 0; }
  [ -f "$KCDIR/status-email" ] || { echo '{"loggedIn":false}'; exit 0; }
  [ -f "$KCDIR/no-refresh" ] || security add-generic-password -U -s "$svc" -a "$USER" \
    -w "$(jq -c '.claudeAiOauth.accessToken |= "ref-" + . | .claudeAiOauth.expiresAt = 9999999999999' <<<"$cur")"
  echo "{\"loggedIn\":true,\"email\":\"$(cat "$KCDIR/status-email")\"}"
  exit 0
fi
printf '%s' "$*" >"$KCDIR/login-args"
[ "$1 $2" = "auth login" ] || exit 0
[ -n "${CLAUDE_CONFIG_DIR:-}" ] || { echo "stub: login without CLAUDE_CONFIG_DIR" >&2; exit 1; }
svc="Claude Code-credentials-$(basename "$CLAUDE_CONFIG_DIR")"
[ -n "${STRAY:-}" ] && svc="Claude Code-credentials" # simulate a version that ignores the config dir
security add-generic-password -U -s "$svc" -a "$USER" \
  -w "{\"claudeAiOauth\":{\"accessToken\":\"new-$PICK\",\"refreshTokenExpiresAt\":9999999999999}}"
jq -n --arg e "$PICK" '{oauthAccount:{emailAddress:$e}}' >"$CLAUDE_CONFIG_DIR/.claude.json"
EOF
# Fake usage API: records argv and stdin, answers with $KCDIR/usage.json.
cat >"$S/stub/curl" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$*" >"$KCDIR/curl-args"
cat >"$KCDIR/curl-stdin"
[ -f "$KCDIR/usage.json" ] || { printf '\n500'; exit 0; }
cat "$KCDIR/usage.json"; printf '\n200'
EOF
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
PICK=a@x "$BIN" use a >/dev/null
ok "manual use of dead account logs in with its email" "$(cat "$KCDIR/login-args")" "auth login --email a@x"
ok "re-login saved fresh token" "$(jq -r .login.claudeAiOauth.accessToken "$KCDIR/claude-switch__a")" new-a@x
ok "re-login then switches" "$(live_token)" new-a@x
ok "dead flag cleared" "$("$BIN" list | grep -c 'LOGIN NEEDED')" 0
ok "temp login item removed" "$(ls "$KCDIR" | grep -c 'credentials-login')" 0

before=$(live_token)
PICK=c@x "$BIN" add extra --login >/dev/null
ok "add --login saves the new account" "$(jq -r .login.claudeAiOauth.accessToken "$KCDIR/claude-switch__extra")" new-c@x
ok "add --login keeps live login" "$(live_token)" "$before"
ok "add --login keeps active" "$("$BIN" current)" a
PICK=c@x "$BIN" add dup --login >/dev/null 2>&1
ok "add --login refuses duplicate email" "$(jq -r '.accounts.dup // "none"' "$HOME/.claude-switch/state.json")" none
PICK=z@x "$BIN" login extra >/dev/null 2>&1
ok "login refuses wrong account" "$(jq -r .login.claudeAiOauth.accessToken "$KCDIR/claude-switch__extra")" new-c@x
before=$(live_token)
STRAY=1 PICK=d@x "$BIN" add stray --login >/dev/null 2>&1
ok "stray login restores live login" "$(live_token)" "$before"
ok "stray login still saves new account" "$(jq -r .login.claudeAiOauth.accessToken "$KCDIR/claude-switch__stray")" new-d@x
"$BIN" remove stray >/dev/null
"$BIN" rename b work >/dev/null
ok "rename leaves active alone" "$("$BIN" current)" a
ok "rename moves saved login" "$(jq -r .login.claudeAiOauth.accessToken "$KCDIR/claude-switch__work")" tokB2
ok "rename drops old login" "$([ -f "$KCDIR/claude-switch__b" ] && echo yes || echo no)" no
"$BIN" rename a main >/dev/null
ok "rename moves active" "$("$BIN" current)" main
ok "rename keeps order" "$(jq -c .order "$HOME/.claude-switch/state.json")" '["main","work","extra"]'
# Usage API for inactive accounts. "work" (b@x) gets a valid access token.
ST="$HOME/.claude-switch/state.json"
jq -c '.login.claudeAiOauth.expiresAt = 9999999999999' "$KCDIR/claude-switch__work" >"$S/t" && mv "$S/t" "$KCDIR/claude-switch__work"
cat >"$KCDIR/usage.json" <<'EOF'
{"five_hour":{"utilization":99.0,"resets_at":"2099-01-01T10:00:00.123456+00:00"},"seven_day":{"utilization":20.0,"resets_at":"2099-01-05T10:00:00+00:00"}}
EOF
"$BIN" list >/dev/null
ok "list reads usage of inactive account" "$(jq -r ".usage.work.five | floor" "$ST")" 99
ok "list parses reset time" "$(jq -r .usage.work.five_reset "$ST")" 4070944800
ok "list skips expired token" "$(jq -r '.usage.extra // "none"' "$ST")" none
ok "token not in curl argv" "$(grep -c tokB2 "$KCDIR/curl-args")" 0
ok "token sent via stdin header" "$(cat "$KCDIR/curl-stdin")" "Authorization: Bearer tokB2"
ok "honest user agent" "$(grep -c 'User-Agent: claude-switch/' "$KCDIR/curl-args")" 1
rm "$KCDIR/curl-args"
"$BIN" next >/dev/null 2>&1
ok "next skips account known exhausted" "$("$BIN" current)" extra
ok "next makes no request" "$([ -f "$KCDIR/curl-args" ] && echo yes || echo no)" no
"$BIN" use main >/dev/null
rm "$KCDIR/usage.json"
"$BIN" list >/dev/null
ok "failed request keeps last numbers" "$(jq -r ".usage.work.five | floor" "$ST")" 99

# Expired inactive token: Claude Code refreshes it in a throwaway config dir.
echo '{"five_hour":{"utilization":10,"resets_at":"2099-01-01T10:00:00Z"}}' >"$KCDIR/usage.json"
before=$(live_token)
echo z@x >"$KCDIR/status-email"
"$BIN" list >/dev/null
ok "refresh refused when Claude Code loads another account" "$(jq -r .login.claudeAiOauth.accessToken "$KCDIR/claude-switch__extra")" new-c@x
echo c@x >"$KCDIR/status-email"
touch "$KCDIR/no-refresh"
"$BIN" list >/dev/null
ok "nothing saved when Claude Code does not refresh" "$(jq -r .login.claudeAiOauth.accessToken "$KCDIR/claude-switch__extra")" new-c@x
rm "$KCDIR/no-refresh"
"$BIN" list >/dev/null
ok "claude refreshes expired inactive token" "$(jq -r .login.claudeAiOauth.accessToken "$KCDIR/claude-switch__extra")" ref-new-c@x
ok "refreshed account gets usage" "$(jq -r '.usage.extra.five | floor' "$ST")" 10
ok "refresh leaves live login" "$(live_token)" "$before"
ok "refresh removes temp keychain items" "$(ls "$KCDIR" | grep -c 'credentials-')" 0
ok "refresh removes temp dirs" "$(ls "$HOME/.claude-switch" | grep -c refresh)" 0

"$BIN" threshold 50 >/dev/null
ok "threshold is saved" "$("$BIN" threshold)" 50
ok "env overrides saved threshold" "$(CLAUDE_SWITCH_THRESHOLD=70 "$BIN" threshold)" 70
"$BIN" threshold 0 >/dev/null 2>&1
ok "threshold rejects 0" "$("$BIN" threshold)" 50
"$BIN" list >"$S/list" 2>&1
ok "list renders" "$?" 0
cat "$S/list"

echo
[ "$fails" = 0 ] && echo "all passed" || { echo "$fails failed"; exit 1; }
