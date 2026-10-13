//go:build !darwin

package platform

import (
	"github.com/PuntoyComaTech/claude-switch/internal/app"
	"github.com/PuntoyComaTech/claude-switch/internal/store"
	"github.com/PuntoyComaTech/claude-switch/internal/sysexec"
)

// On Linux and Windows Claude Code keeps its own login in a 0600 file, so
// saved logins live in 0600 files in a private directory too.
func profiles(p Paths, _ sysexec.Runner) app.ProfileStore {
	return store.FileProfiles{Dir: p.Profiles}
}
