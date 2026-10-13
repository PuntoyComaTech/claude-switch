package notify

import (
	"strconv"

	"github.com/PuntoyComaTech/claude-switch/internal/sysexec"
)

func command(msg string) sysexec.Cmd {
	script := "display notification " + strconv.Quote(msg) + " with title " + strconv.Quote(title)
	return sysexec.Cmd{Name: "osascript", Args: []string{"-e", script}}
}
