# claude-switch: use several Claude accounts in Claude Code

**claude-switch** lets you use several Claude subscriptions (Pro, Max or Team) in [Claude Code](https://code.claude.com) and switches between them automatically when one hits its 5-hour or weekly usage limit. No logging out, no restarting your sessions. Works on **macOS, Linux and Windows**.

```
  ACCOUNT      EMAIL                SESSION (5h)                       WEEKLY (7d)
● work         me@company.com       ███░░░░░░░  32%  resets in 4h 5m   █████████░  90%  resets in 1d 5h
  personal     me@gmail.com         █░░░░░░░░░  12%  resets in 2h 40m  █░░░░░░░░░   8%  resets in 5d 2h

switches automatically at 95% of either window
```

- **Automatic.** Claude Code already reports your 5-hour and weekly usage to its status line. claude-switch reads it there and moves every session to the next account at 95%.
- **Nothing to log into again.** Running sessions pick up the new account on their next request.
- **Safe with your login.** Adding an account logs in inside an isolated config, so your current sessions are never touched. Token refreshes are always done by the official Claude Code binary.
- **One small binary.** No runtime, no dependencies, free to install.

## Install

macOS and Linux:

```sh
curl -fsSL https://raw.githubusercontent.com/PuntoyComaTech/claude-switch/main/install.sh | sh
```

Windows (PowerShell):

```powershell
irm https://raw.githubusercontent.com/PuntoyComaTech/claude-switch/main/install.ps1 | iex
```

With Go: `go install github.com/PuntoyComaTech/claude-switch/cmd/claude-switch@latest`, then run `claude-switch setup`.

The installer downloads the release for your system, checks its SHA-256 checksum, puts `claude-switch` in `~/.local/bin` (Windows: `%LOCALAPPDATA%\Programs\claude-switch`) and hooks it into the Claude Code status line:

- **No status line yet:** it sets one that shows account, model and usage.
- **You already have one:** it prints one line to add to your script. It never edits your script.

## Set up your accounts

```sh
claude-switch add personal          # the account Claude Code is logged in with now
claude-switch add work --login      # opens the browser; pick the other account
claude-switch list
```

Repeat `add <name> --login` for each extra subscription. Then use Claude Code as usual: when the active account reaches the threshold, the next response triggers the switch.

## Commands

| Command | What it does |
|---|---|
| `claude-switch list` | Accounts, usage, which one is active, which need a login |
| `claude-switch use <name>` | Switch now. Running sessions follow on their next request |
| `claude-switch next` | Switch to the next account that is not at its limit |
| `claude-switch add <name>` | Save the account you are logged in with |
| `claude-switch add <name> --login` | Log in to another account and save it, without switching |
| `claude-switch login <name>` | Log in again to a saved account whose login expired |
| `claude-switch rename <old> <new>` | Rename an account |
| `claude-switch remove <name>` | Forget an account |
| `claude-switch current` | Print the active account name |
| `claude-switch threshold [n]` | Show or set the switch threshold (default 95) |
| `claude-switch setup` | Hook claude-switch into the Claude Code status line |
| `claude-switch statusline` | Built-in status line: `account · model · 5h % · 7d %` |
| `claude-switch report` | Read status line JSON on stdin and switch when over the limit |

Environment: `CLAUDE_SWITCH_THRESHOLD` overrides the threshold for one command; `CLAUDE_SWITCH_DIR` moves the data folder.

## Using your own status line

Add this line right after your script reads stdin into `$input`:

```sh
command -v claude-switch >/dev/null && { printf '%s' "$input" | claude-switch report >/dev/null 2>&1 & }
```

Show the active account with `$(claude-switch current)`. Also set `"refreshInterval": 120` on `statusLine` in `~/.claude/settings.json`, so idle sessions still switch; it reruns your local script only, with no requests.

## How it works

Claude Code keeps its login in one place and the signed-in account (`oauthAccount`) in `~/.claude.json`. claude-switch keeps a copy of both for each account. To switch it saves the current login first (Claude Code rotates its tokens), writes the target login, writes the target `oauthAccount` under Claude Code's own config lock, and touches `~/.claude/.credentials.json`, which running sessions watch.

| | Claude Code's login | claude-switch's saved accounts | Data folder |
|---|---|---|---|
| macOS | Keychain item `Claude Code-credentials` | Keychain, service `claude-switch` | `~/.claude-switch` |
| Linux | `~/.claude/.credentials.json` (0600) | `profiles/<name>.json` (0600) | `~/.local/share/claude-switch` |
| Windows | `%USERPROFILE%\.claude\.credentials.json` | `profiles\<name>.json` | `%LOCALAPPDATA%\claude-switch` |

`CLAUDE_CONFIG_DIR` and `CLAUDE_SECURESTORAGE_CONFIG_DIR` are honored the same way Claude Code honors them.

On macOS secrets reach `/usr/bin/security` through stdin, never as arguments, except for Claude Code's own item when it is too large for one stdin line; Claude Code itself does the same. Items are always written through `security`, so Claude Code never shows a Keychain password prompt.

**Usage of inactive accounts.** The status line only carries the active account's usage. `list` reads the others from `https://api.anthropic.com/api/oauth/usage`, the endpoint behind Claude Code's `/usage`: one request per inactive account, only when you run `list`. An expired token is refreshed by running the official `claude auth status` in a throwaway config. It identifies itself as `claude-switch/<version>`. The endpoint is undocumented and may change; on failure `list` keeps the last numbers and logs the error.

## FAQ

**Can I use multiple Claude accounts in Claude Code?** Yes. Save each one with `claude-switch add`, then switch with `claude-switch use <name>` or let it switch automatically at the usage limit.

**What happens when I hit the Claude usage limit?** On the next response, claude-switch moves Claude Code to the next account below the threshold. If every account is at its limit it stays put and shows a notification.

**Do I need to restart Claude Code after switching?** No. Running sessions use the new account on their next request.

**Does it work with Claude Pro, Max and Team?** Yes, with any claude.ai subscription login. API keys are not affected.

**Does it work on Windows?** Yes, when Claude Code stores its login in `.credentials.json` (the default). If Claude Code is set to use Windows Credential Manager, claude-switch says so and stops; please open an issue.

**Is it safe?** It never sends your tokens anywhere except Anthropic's own usage endpoint, never logs secrets, and keeps saved logins in the same kind of storage Claude Code uses for its own.

Check Anthropic's terms for your plan before using several subscriptions.

## Limits

- All running sessions switch together, since they share one login.
- If you `/login` manually to an account that is not saved, switching stops until you `add` it, so that login is never overwritten.
- A login revoked on the server cannot be detected without a request. Claude Code shows an auth error; run `claude-switch login <name>`.

## Uninstall

```sh
curl -fsSL https://raw.githubusercontent.com/PuntoyComaTech/claude-switch/main/install.sh | sh -s -- --uninstall
```

This removes the binary. To also forget saved accounts, run `claude-switch remove <name>` for each one first, then delete the data folder from the table above. The login Claude Code is using stays as it is.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). AI coding agents: read [AGENTS.md](AGENTS.md).

## License

MIT
