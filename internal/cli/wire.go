package cli

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/PuntoyComaTech/claude-switch/internal/app"
	"github.com/PuntoyComaTech/claude-switch/internal/claudecode"
	"github.com/PuntoyComaTech/claude-switch/internal/notify"
	"github.com/PuntoyComaTech/claude-switch/internal/platform"
	"github.com/PuntoyComaTech/claude-switch/internal/store"
	"github.com/PuntoyComaTech/claude-switch/internal/sysexec"
	"github.com/PuntoyComaTech/claude-switch/internal/usageapi"
)

// env holds what the composition root needs from the process.
type env struct {
	home  string
	paths platform.Paths
}

func loadEnv() (env, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return env{}, err
	}
	return env{home: home, paths: platform.ResolvePaths(home)}, nil
}

// settingsPath is Claude Code's user settings file.
func (e env) settingsPath() string {
	dir := os.Getenv("CLAUDE_CONFIG_DIR")
	if dir == "" {
		dir = filepath.Join(e.home, ".claude")
	}
	return filepath.Join(dir, "settings.json")
}

// newService is the composition root: it picks the adapters for this OS.
// out receives user-facing messages.
func newService(e env, version string, out io.Writer) (*app.Service, error) {
	if err := os.MkdirAll(e.paths.Dir, 0o700); err != nil {
		return nil, err
	}
	run := sysexec.OS{}
	homes, err := claudecode.NewHomes(claudecode.Config{
		Env:     os.LookupEnv,
		HomeDir: e.home,
		User:    platform.Username(),
		TempDir: e.paths.Temp,
		Run:     run,
		Claude:  os.Getenv("CLAUDE_SWITCH_CLAUDE_BIN"),
	})
	if err != nil {
		return nil, err
	}
	url := os.Getenv("CLAUDE_SWITCH_USAGE_URL")
	if url == "" {
		url = usageapi.DefaultURL
	}
	threshold, _ := strconv.Atoi(os.Getenv("CLAUDE_SWITCH_THRESHOLD"))
	return app.New(app.Deps{
		Profiles: platform.Profiles(e.paths, run),
		State:    store.StateFile{Path: e.paths.State},
		Lock:     store.FileLock{Path: e.paths.Lock},
		Homes:    homes,
		Usage: usageapi.Client{
			URL: url, HTTP: &http.Client{}, UserAgent: "claude-switch/" + version, Now: time.Now,
		},
		Notifier:  notify.Desktop{Run: run},
		Logger:    store.LogFile{Path: e.paths.Log},
		Out:       out,
		Threshold: threshold,
	}), nil
}
