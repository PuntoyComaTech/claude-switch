package platform

import (
	"path/filepath"

	"github.com/PuntoyComaTech/claude-switch/internal/app"
	"github.com/PuntoyComaTech/claude-switch/internal/keychain"
	"github.com/PuntoyComaTech/claude-switch/internal/store"
	"github.com/PuntoyComaTech/claude-switch/internal/sysexec"
)

// ~/.claude-switch is where the bash version kept its state, so existing
// installs keep their accounts.
func defaultDir(home string) string { return filepath.Join(home, ".claude-switch") }

// On macOS saved logins live in the Keychain, next to Claude Code's own.
func profiles(_ Paths, run sysexec.Runner) app.ProfileStore {
	return store.KeychainProfiles{KC: keychain.Keychain{Run: run}}
}
