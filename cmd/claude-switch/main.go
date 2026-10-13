// Command claude-switch switches Claude Code between Claude subscription
// accounts (Pro, Max, Team), automatically when one hits its usage limit.
//
// Install: https://github.com/PuntoyComaTech/claude-switch#install
package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/PuntoyComaTech/claude-switch/internal/cli"
)

// version is set at build time by GoReleaser (-ldflags "-X main.version=...").
var version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	code := cli.Execute(ctx, version)
	stop()
	os.Exit(code)
}
