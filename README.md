# claude-switch

Use several Claude subscriptions (Pro / Max) in Claude Code on macOS. When one reaches its usage limit, every session moves to the next one on its own, without logging in again and without restarting.

```
  ACCOUNT      EMAIL                SESSION (5h)                       WEEKLY (7d)
● trabajo      me@company.com       ███░░░░░░░  32%  resets in 4h 5m   █████████░  90%  resets in 1d 5h
  personal     me@gmail.com         █░░░░░░░░░  12%  resets in 2h 40m  █░░░░░░░░░   8%  resets in 5d 2h

switches automatically at 96% of either window
```

- **Automatic.** Claude Code already sends your 5-hour and weekly usage to the status line. claude-switch reads it there and switches at 95%.
- **No extra traffic.** It never calls Anthropic APIs and never refreshes tokens itself. Claude Code keeps doing that.
- **Adding an account does not touch your sessions.** The browser login for a new account happens in an isolated config, so running sessions stay where they are.
- **One bash script.** Needs `jq`, which ships with recent macOS.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/PuntoyComaTech/claude-switch/main/install.sh | bash
```

This puts `claude-switch` in `~/.local/bin` and hooks it into your Claude Code status line:

- **No status line yet:** it installs one that shows account, model and usage.
- **You already have one:** it prints one line to paste into your script. It never edits your script.

Prefer a clone, so `git pull` updates it?

```sh
git clone https://github.com/PuntoyComaTech/claude-switch.git
cd claude-switch && ./install.sh
```

## Set up your accounts

```sh
claude-switch add personal          # the account Claude Code is logged in with now
claude-switch add trabajo --login   # opens the browser; pick the other account
claude-switch list
```

Repeat `add <name> --login` for each extra subscription. Names are any single word.

On first use macOS may ask whether `security` can access the Keychain. Choose **Always Allow**.

That is all. Use Claude Code as usual. When the active account reaches the threshold, the next response triggers the switch and the status line shows the new account.

## Everyday commands

| Command | What it does |
|---|---|
| `claude-switch list` | Accounts, last known usage, which one is active, which need a login |
| `claude-switch use <name>` | Switch now. Running sessions follow on their next request |
| `claude-switch next` | Switch to the next account that is not at its limit |
| `claude-switch add <name>` | Save the account you are logged in with |
| `claude-switch add <name> --login` | Log in to another account and save it, without switching |
| `claude-switch login <name>` | Log in again to a saved account whose login expired |
| `claude-switch rename <old> <new>` | Rename an account |
| `claude-switch remove <name>` | Forget an account |
| `claude-switch current` | Print the active account name |
| `claude-switch threshold [n]` | Show or set the switch threshold |

### Settings

```sh
claude-switch threshold 96    # switch at 96% of the 5h or weekly window (default 95)
claude-switch threshold       # show the current value
```

`CLAUDE_SWITCH_THRESHOLD` overrides the saved value for one command. `CLAUDE_SWITCH_DIR` moves the state folder (default `~/.claude-switch`).

Logs: `~/.claude-switch/log`.

## Using your own status line

Add this line after your script reads stdin into `$input`:

```sh
command -v claude-switch >/dev/null && { printf '%s' "$input" | claude-switch report >/dev/null 2>&1 & }
```

To show the active account, add `$(claude-switch current)` to your output. [statusline-example.sh](statusline-example.sh) is a complete example.

## How it works

Claude Code keeps its login in the macOS Keychain (item `Claude Code-credentials`) and the signed-in account in `~/.claude.json` (`oauthAccount`). claude-switch keeps a copy of both for each account in its own Keychain items (service `claude-switch`). To switch it:

1. Saves the current login first, because Claude Code rotates its tokens.
2. Writes the target login into Claude Code's Keychain item. MCP logins stored in the same item are kept.
3. Writes the target `oauthAccount` into `~/.claude.json` atomically.
4. Touches `~/.claude/.credentials.json` so running sessions re-read the Keychain.

`report` runs from the status line after each response. It records the active account's usage and calls `next` when either window reaches the threshold. Right after a switch, a session keeps showing the old account's numbers until its next response. Repeated numbers from the same session are therefore ignored.

`add --login` runs `claude auth login` with `CLAUDE_CONFIG_DIR` pointing at a temporary folder. Claude Code stores that login in a separate Keychain item. claude-switch copies it and deletes the temporary item and folder. If a Claude Code version wrote the main item anyway, claude-switch restores the original.

## Expired logins

claude-switch checks the saved refresh-token expiry date locally, with no request:

- `use` / `next` from a terminal start the browser login for that account with the email pre-filled.
- Automatic switching skips that account and shows a macOS notification asking you to run `claude-switch login <name>`.

A login revoked on the server, for example after signing out everywhere on claude.ai, cannot be detected without a request. Claude Code will show an auth error; run `claude-switch login <name>`.

## Limits

- macOS only, default Claude Code config dir only.
- All running sessions switch together, since they share the Keychain.
- If you `/login` manually to an account that is not saved, switching stops until you `add` it, so that login is never overwritten.
- If every account is at its limit, it stays on the current one and notifies you.
- Usage numbers are as fresh as the last response in any session.
- Keychain writes pass the login JSON to `/usr/bin/security` as an argument. It is visible to your own user's processes for a moment.

Check Anthropic's terms for your plan before using several subscriptions.

## Uninstall

```sh
curl -fsSL https://raw.githubusercontent.com/PuntoyComaTech/claude-switch/main/install.sh | bash -s -- --uninstall
```

Removes the command, the saved logins and the state. The login Claude Code is currently using stays as it is.

## Development

```sh
./tests/sandbox.sh
```

Runs every flow against a fake Keychain in a temporary `HOME`. It never touches your real login. See [AGENTS.md](AGENTS.md) for conventions.

## License

MIT
