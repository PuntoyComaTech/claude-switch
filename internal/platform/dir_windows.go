package platform

import (
	"os"
	"path/filepath"
)

// %LOCALAPPDATA% is private to the user and, unlike Roaming, never synced to
// other machines, so saved logins stay on this computer.
func defaultDir(home string) string {
	if d := os.Getenv("LOCALAPPDATA"); d != "" {
		return filepath.Join(d, "claude-switch")
	}
	return filepath.Join(home, "AppData", "Local", "claude-switch")
}
