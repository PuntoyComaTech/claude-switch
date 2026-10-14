//go:build !darwin && !linux && !windows

package notify

import "github.com/PuntoyComaTech/claude-switch/internal/sysexec"

// command uses notify-send where it may exist (BSDs).
func command(msg string) sysexec.Cmd {
	return sysexec.Cmd{Name: "notify-send", Args: []string{title, msg}}
}
