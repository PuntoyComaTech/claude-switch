// Package notify shows desktop notifications with the tools each OS ships:
// osascript on macOS, notify-send on Linux, a PowerShell toast on Windows.
// A missing tool is silently ignored; the message is also in the log.
package notify

import (
	"context"
	"time"

	"github.com/PuntoyComaTech/claude-switch/internal/app"
	"github.com/PuntoyComaTech/claude-switch/internal/sysexec"
)

const title = "claude-switch"

// Desktop implements app.Notifier.
type Desktop struct{ Run sysexec.Runner }

var _ app.Notifier = Desktop{}

// Notify implements app.Notifier.
func (d Desktop) Notify(msg string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = d.Run.Run(ctx, command(msg))
}
