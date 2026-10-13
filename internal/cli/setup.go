package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/PuntoyComaTech/claude-switch/internal/domain"
	"github.com/PuntoyComaTech/claude-switch/internal/fsutil"
)

// statusLineCommand is what Claude Code runs as its status line once set up.
const statusLineCommand = "claude-switch statusline"

// reportHook is the line users add to a status line script they already have.
const reportHook = `command -v claude-switch >/dev/null && { printf '%s' "$input" | claude-switch report >/dev/null 2>&1 & }`

// setupStatusLine hooks claude-switch into Claude Code's settings.json. An
// existing status line is never replaced; the user gets the line to add.
func setupStatusLine(w io.Writer, settingsPath string) error {
	settings := map[string]json.RawMessage{}
	b, err := os.ReadFile(settingsPath)
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		return fmt.Errorf("read %s: %w", settingsPath, err)
	default:
		if err := json.Unmarshal(b, &settings); err != nil {
			return fmt.Errorf("parse %s: %w", settingsPath, err)
		}
	}

	var current struct {
		Command string `json:"command"`
	}
	if raw, ok := settings["statusLine"]; ok {
		_ = json.Unmarshal(raw, &current)
	}
	switch {
	case current.Command == "":
		line, _ := json.Marshal(map[string]any{"type": "command", "command": statusLineCommand, "refreshInterval": 120})
		settings["statusLine"] = line
		out, err := domain.MarshalIndent(settings)
		if err != nil {
			return err
		}
		if len(b) > 0 {
			if err := fsutil.WriteFileAtomic(settingsPath+".bak.claude-switch", b, 0o600); err != nil {
				return err
			}
		}
		if err := fsutil.WriteFileAtomic(settingsPath, out, fsutil.ModeOr(settingsPath, 0o644)); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(w, "status line: installed in %s\n", settingsPath)
	case hooked(current.Command):
		_, _ = fmt.Fprintln(w, "status line: already hooked")
	default:
		_, _ = fmt.Fprintf(w, "status line: you already have one (%s).\n", current.Command)
		_, _ = fmt.Fprintf(w, "Add this line to it, right after it reads stdin into $input:\n\n  %s\n\n", reportHook)
	}
	return nil
}

// hooked reports whether a status line command already feeds claude-switch.
func hooked(command string) bool {
	if strings.Contains(command, "claude-switch") {
		return true
	}
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return false
	}
	script := fields[len(fields)-1]
	if strings.HasPrefix(script, "~") {
		home, _ := os.UserHomeDir()
		script = filepath.Join(home, script[1:])
	}
	b, err := os.ReadFile(script)
	return err == nil && strings.Contains(string(b), "claude-switch report")
}
