// Package platform decides, per operating system, where claude-switch keeps
// its data and which profile store it uses.
package platform

import (
	"os"
	"os/user"
	"path/filepath"

	"github.com/PuntoyComaTech/claude-switch/internal/app"
	"github.com/PuntoyComaTech/claude-switch/internal/sysexec"
)

// Paths are claude-switch's own files.
type Paths struct {
	Dir      string // root, overridable with CLAUDE_SWITCH_DIR
	State    string // state.json
	Lock     string // process lock
	Log      string // log
	Profiles string // saved profiles (unused where the Keychain holds them)
	Temp     string // throwaway Claude Code configs
}

// ResolvePaths returns the data paths for this OS and environment.
func ResolvePaths(home string) Paths {
	dir := os.Getenv("CLAUDE_SWITCH_DIR")
	if dir == "" {
		dir = defaultDir(home)
	}
	return Paths{
		Dir:      dir,
		State:    filepath.Join(dir, "state.json"),
		Lock:     filepath.Join(dir, "lock.flock"),
		Log:      filepath.Join(dir, "log"),
		Profiles: filepath.Join(dir, "profiles"),
		Temp:     filepath.Join(dir, "tmp"),
	}
}

// Username is the account name Claude Code uses for its Keychain item.
func Username() string {
	if u := os.Getenv("USER"); u != "" {
		return u
	}
	if u, err := user.Current(); err == nil {
		return filepath.Base(u.Username) // Windows: DOMAIN\name
	}
	return ""
}

// Profiles returns the profile store for this OS.
func Profiles(p Paths, run sysexec.Runner) app.ProfileStore { return profiles(p, run) }
