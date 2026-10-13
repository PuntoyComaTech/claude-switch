# Contributing to claude-switch

Thanks for helping. Issues and pull requests are welcome.

## Setup

You need Go (see `go.mod`) and these tools, installed the way each project recommends:

| Tool | Install |
|---|---|
| golangci-lint v2 | `curl -sSfL https://golangci-lint.run/install.sh \| sh -s -- -b $(go env GOPATH)/bin` |
| gofumpt | `go install mvdan.cc/gofumpt@latest` |
| govulncheck | `go install golang.org/x/vuln/cmd/govulncheck@latest` |
| shellcheck | your package manager (`brew install shellcheck`, `apt install shellcheck`) |
| goreleaser (only to test releases) | see https://goreleaser.com/install |

Make sure `$(go env GOPATH)/bin` is on your `PATH`.

## Workflow

```sh
make check     # format check, lint on macOS/Linux/Windows, tests with -race, govulncheck, shellcheck
make test      # tests only
make build     # ./bin/claude-switch
make snapshot  # local release build in ./dist (nothing is published)
```

Tests never touch your real login: `internal/app` uses in-memory fakes, and the macOS Keychain test only runs with `CLAUDE_SWITCH_KEYCHAIN_TEST=1` against a throwaway item.

Read [AGENTS.md](AGENTS.md) > Changing the code for the architecture rules. They are enforced by the linter, so `make lint` tells you when a package reaches into a layer it should not.

## Pull requests

- One change per PR, with a test.
- Conventional commit titles (`feat:`, `fix:`, `docs:` ...): the changelog is built from them.
- CI must pass on macOS, Linux and Windows.

## Releases

Maintainers tag `vX.Y.Z` on `main`; the release workflow builds every platform with GoReleaser and publishes the archives and `checksums.txt` that the install scripts use.
