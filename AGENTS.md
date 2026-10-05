# AGENTS.md

Instructions for AI agents installing, configuring or changing claude-switch.

## Installing for a user

1. Check the platform: macOS only. Run `uname` and `command -v jq claude`. If `jq` is missing, ask before running `brew install jq`.
2. Install:
   ```sh
   curl -fsSL https://raw.githubusercontent.com/PuntoyComaTech/claude-switch/main/install.sh | bash
   ```
   Or run `./install.sh` from a clone.
3. Read the installer output:
   - `status line: installed` or `already hooked`: nothing to do.
   - `you already have one`: show the user the printed line and the script it goes in. Ask before editing their status line script. If they agree, insert the line right after the script reads stdin into `$input`, then pipe sample JSON to the script to confirm it still renders.
4. Accounts need the human. Tell the user to run these themselves:
   ```sh
   claude-switch add <name>            # the account logged in now
   claude-switch add <name> --login    # each other account; opens a browser
   ```
   Account names are single words the user picks. Do not invent them.
   macOS may show a Keychain prompt for `security`; the user should choose Always Allow.
5. Verify with `claude-switch list`. Every account must show its email.

## Rules

- Never read, print, copy or log Keychain secrets or `~/.claude/.credentials.json`. Do not run `security find-generic-password -w` yourself; claude-switch does it.
- Never run `claude auth login` or `/login` for the user. It replaces the login of every running session. Use `claude-switch add <name> --login`, which is isolated.
- Do not edit `~/.claude.json` or the `Claude Code-credentials` Keychain item by hand.
- Do not change the threshold or the status line without the user's request.

## Troubleshooting

| Symptom | Fix |
|---|---|
| `you are logged in as 'x', which is not saved` | The user logged in manually. Have them run `claude-switch add <name>` |
| `LOGIN NEEDED` in `list` | Have them run `claude-switch login <name>` |
| No automatic switch | Check `~/.claude-switch/log`. Confirm the status line runs `claude-switch report`. `rate_limits` only exists for Pro/Max after the first response |
| `another switch is in progress` | Wait a few seconds. A lock older than 60s is cleared automatically |

## Changing the code

- `bin/claude-switch` is the whole tool. Keep it one bash script, `set -euo pipefail`, deps limited to macOS built-ins plus `jq`.
- Pass secrets to `jq` through stdin, never argv.
- Every Keychain write is read back and verified (`kc_write`). Keep that.
- Run `./tests/sandbox.sh` before committing. It stubs `security`, `claude` and `osascript` in a temporary `HOME`. Add a case for every behavior you change.
- Conventional commits.
