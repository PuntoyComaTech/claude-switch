package claudecode

import (
	"context"
	"fmt"
	"os"

	"github.com/PuntoyComaTech/claude-switch/internal/app"
	"github.com/PuntoyComaTech/claude-switch/internal/sysexec"
)

// Homes implements app.Homes.
type Homes struct {
	live    Layout
	tempDir string
	user    string
	run     sysexec.Runner
	claude  string
}

var _ app.Homes = (*Homes)(nil)

// Config builds Homes.
type Config struct {
	Env     Env
	HomeDir string
	User    string
	// TempDir is where throwaway configurations are created.
	TempDir string
	Run     sysexec.Runner
	// Claude is the Claude Code binary, "claude" by default.
	Claude string
}

// NewHomes resolves the live configuration.
func NewHomes(c Config) (*Homes, error) {
	if c.Claude == "" {
		c.Claude = "claude"
	}
	live := LiveLayout(c.Env, c.HomeDir, c.User)
	if err := checkSupported(c.Env, live); err != nil {
		return nil, err
	}
	return &Homes{live: live, tempDir: c.TempDir, user: c.User, run: c.Run, claude: c.Claude}, nil
}

// Live implements app.Homes.
func (h *Homes) Live() app.Home { return h.home(h.live) }

// Temp implements app.Homes.
func (h *Homes) Temp() (app.Home, func(), error) {
	if err := os.MkdirAll(h.tempDir, 0o700); err != nil {
		return nil, nil, fmt.Errorf("create %s: %w", h.tempDir, err)
	}
	dir, err := os.MkdirTemp(h.tempDir, "cc-")
	if err != nil {
		return nil, nil, fmt.Errorf("create temp config: %w", err)
	}
	home := h.home(TempLayout(dir, h.user))
	cleanup := func() {
		home.login.cleanup(context.Background())
		_ = os.RemoveAll(dir)
	}
	return home, cleanup, nil
}

// LiveLayout returns the resolved live layout (used by setup and tests).
func (h *Homes) LiveLayout() Layout { return h.live }

func (h *Homes) home(l Layout) *Home {
	return &Home{layout: l, login: newLoginStore(l, h.run), run: h.run, claude: h.claude}
}
