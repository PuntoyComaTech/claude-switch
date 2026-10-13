package notify

import "github.com/PuntoyComaTech/claude-switch/internal/sysexec"

func command(msg string) sysexec.Cmd {
	return sysexec.Cmd{Name: "notify-send", Args: []string{title, msg}}
}
