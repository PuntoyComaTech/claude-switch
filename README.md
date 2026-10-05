# claude-switch

Use several Claude subscriptions (Pro / Max) with Claude Code on macOS, and switch to the next one automatically when the current one hits its limit.

- **One command to switch.** `claude-switch use work` and every Claude Code session, including ones already running, moves to that account. No browser login each time.
- **Automatic.** Your status line already receives your 5h and weekly usage from Claude Code. claude-switch reads it and moves to the next account at 95%.
- **No extra traffic.** It never calls Anthropic's APIs or refreshes tokens itself. Usage comes from the status line JSON, and Claude Code refreshes tokens as usual.
- **Small.** One bash script. Needs `jq` (ships with macOS 15+).

## How it works

Claude Code keeps its login in the macOS Keychain (`Claude Code-credentials`) and the signed-in account in `~/.claude.json` (`oauthAccount`). claude-switch saves a copy of both per account in the Keychain (service `claude-switch`) and swaps them in place:

1. Saves the live login of the active account first, because Claude Code rotates its tokens.
2. Writes the target login into the Keychain item. MCP logins in the same item are kept.
3. Writes the target `oauthAccount` into `~/.claude.json` atomically.
4. Touches `~/.claude/.credentials.json`, which makes running sessions re-read the Keychain.

`report` runs from your status line on every update. It records the active account's usage and calls `next` when the 5h or weekly window reaches the threshold. Right after a switch a session still shows the old account's numbers until its next response, so repeated numbers from the same session are ignored.

## Install

```sh
git clone https://github.com/PuntoyComaTech/claude-switch.git
cd claude-switch
./install.sh            # symlinks bin/claude-switch into ~/.local/bin
```

Save each account once:

```sh
claude-switch add personal     # the account you are logged in with now
claude auth login              # log in with the other account in the browser
claude-switch add work
claude-switch list
```

The first Keychain access may show a macOS prompt for `security`. Choose **Always Allow**.

### Automatic switching

Add this to your status line script, after it reads stdin into `$input`:

```sh
command -v claude-switch >/dev/null && { printf '%s' "$input" | claude-switch report >/dev/null 2>&1 & }
```

Optional: show the active account in the status line with `$(claude-switch current)`.

No status line yet? See [statusline-example.sh](statusline-example.sh) and add to `~/.claude/settings.json`:

```json
"statusLine": { "type": "command", "command": "bash /path/to/statusline-example.sh" }
```

## Commands

| Command | What it does |
|---|---|
| `add <name>` | Save the account you are logged in with |
| `use <name>` | Switch to a saved account |
| `next` | Switch to the next account that is not at its limit |
| `login <name>` | Log in again to a saved account and save it |
| `list` | Accounts, last known usage, and which need a login |
| `current` | Print the active account |
| `remove <name>` | Delete a saved account |
| `report` | Read status line JSON on stdin, switch when over the limit |

Settings: `CLAUDE_SWITCH_THRESHOLD` (default `95`), `CLAUDE_SWITCH_DIR` (default `~/.claude-switch`). Logs go to `~/.claude-switch/log`.

## Expired logins

If an account goes unused long enough for its refresh token to expire, claude-switch sees it from the saved expiry date, with no request:

- `use` / `next` from a terminal run `claude auth login --email <that account>`. You only confirm in the browser.
- Automatic switching skips that account and shows a macOS notification asking you to run `claude-switch login <name>`.

A token revoked on the server (for example after logging out on claude.ai) cannot be detected without a request. Claude Code will report the auth error; run `claude-switch login <name>`.

## Limits

- macOS only. Uses the default config dir, not `CLAUDE_CONFIG_DIR`.
- All running sessions switch together, since they share the Keychain.
- A manual `/login` with an account that is not saved blocks switching until you `add` it, so it is never overwritten by mistake.
- When every account is at its limit it stays on the current one.
- Usage numbers are only as fresh as the last response in any session.
- Keychain writes pass the login JSON as an argument to `/usr/bin/security`, so it is visible to your own user's processes for a moment.

Check Anthropic's terms for your plan before using several subscriptions.

## Test

```sh
./tests/sandbox.sh
```

Runs every flow against a fake Keychain in a temporary `HOME`. It never touches your real login.

## License

MIT
