# AGENTS.md

Instructions for AI coding agents that install, configure or change claude-switch.

claude-switch is a single Go binary that saves several Claude subscription logins (Pro, Max, Team) and swaps the one Claude Code uses, automatically when the active account reaches its 5-hour or weekly usage limit. It runs on macOS, Linux and Windows.

## Install it for a user

1. Run the installer for their OS:
   - macOS / Linux: `curl -fsSL https://raw.githubusercontent.com/PuntoyComaTech/claude-switch/main/install.sh | sh`
   - Windows (PowerShell): `irm https://raw.githubusercontent.com/PuntoyComaTech/claude-switch/main/install.ps1 | iex`
   - Go users: `go install github.com/PuntoyComaTech/claude-switch/cmd/claude-switch@latest && claude-switch setup`
2. Read the `status line:` line it prints:
   - `installed` or `already hooked`: done.
   - `you already have one`: show the user the printed line. Ask before editing their status line script. If they agree, put it right after the script reads stdin into `$input`, then pipe sample JSON through the script to confirm it still renders.
3. Saving accounts needs the human (a browser login). Tell them to run:
   ```sh
   claude-switch add <name>            # the account logged in now
   claude-switch add <name> --login    # each other account
   ```
   The user picks the names. Do not invent them.
4. Verify with `claude-switch list`: every account shows its email.

## Never

- Read, print, copy or log secrets: Keychain items, `.credentials.json`, saved profiles. Do not run `security find-generic-password -w`.
- Run `claude auth login` or `/login` for the user. It replaces the login of every running session. `claude-switch add <name> --login` is the isolated way.
- Edit `~/.claude.json`, `.credentials.json` or the `Claude Code-credentials` Keychain item by hand.
- Change the threshold or the status line without being asked.

## Troubleshooting

| Symptom | Fix |
|---|---|
| `you are logged in as 'x', which is not saved` | User logged in manually: `claude-switch add <name>` |
| `LOGIN NEEDED` in `list` | `claude-switch login <name>` |
| No automatic switch | Read `log` in the data folder (README > How it works). Check the status line runs `claude-switch statusline` or `claude-switch report`. `rate_limits` exists only for subscriptions, after the first response |
| `another switch is in progress` | Retry in a few seconds; the lock dies with its process |
| A switch takes about 10 s | Claude Code left `~/.claude.json.lock` behind; claude-switch waits for it to go stale. Expected |
| `Windows Credential Manager ... not support yet` | Claude Code uses its Credential Manager backend; unsupported. Point to the issue tracker |

## Change the code

Layers, enforced by `depguard` in `.golangci.yml` (the lint fails on a violation):

| Package | Responsibility |
|---|---|
| `cmd/claude-switch` | entry point: signals, exit code |
| `internal/cli` | cobra commands, rendering, composition root (`wire.go`) |
| `internal/app` | use cases; imports only `domain` and talks to the system through `ports.go` |
| `internal/domain` | pure rules (state, profiles, credentials, report). No I/O |
| `internal/claudecode` | Claude Code's login and config (Keychain on macOS, file elsewhere) |
| `internal/store` | `state.json`, saved profiles, process lock, log |
| `internal/keychain` | macOS Keychain through `/usr/bin/security` |
| `internal/usageapi` | Anthropic OAuth usage endpoint |
| `internal/notify` | desktop notifications per OS |
| `internal/platform` | per-OS data paths and profile store |
| `internal/fsutil` | atomic writes, Claude Code's mkdir config lock |
| `internal/sysexec` | command runner interface, so tests fake `security` and `claude` |

Rules:

- Put rules in `domain`, flows in `app`, system access in an adapter behind a port. No `os`, `os/exec` or `net` in `app` or `domain`.
- Per-OS code: `_darwin.go` / `_windows.go` suffixes or `//go:build` tags. All three OS must build and lint.
- macOS Keychain only through `internal/keychain`: `security -i` with the secret hex-encoded on stdin, argv only when the line exceeds 4032 bytes (Claude Code does the same). Never the Security framework or go-keyring: items they create make Claude Code show password prompts. Read back and verify every write.
- Mirror Claude Code's storage rules exactly (`internal/claudecode/layout.go`): `CLAUDE_CONFIG_DIR`, `CLAUDE_SECURESTORAGE_CONFIG_DIR`, the hashed Keychain suffix, legacy `.config.json`, and touching `.credentials.json` after a login change.
- Write `.claude.json` only while holding its `.lock` directory, and keep unknown keys.
- No secrets in argv, logs, errors or tests.
- Keep `state.json` readable by existing installs (same schema as the bash version).
- Test every behavior change. `internal/app` tests use the fakes in `fakes_test.go`; tests never touch the real login.
- `make check` must pass before committing: gofumpt, golangci-lint for darwin/linux/windows, `go test -race`, govulncheck, shellcheck.
- Conventional commits (`feat:`, `fix:`, `docs:`, `test:`, `refactor:`, `chore:`, `ci:`); the changelog is built from them.
