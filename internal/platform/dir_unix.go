//go:build !darwin && !windows

package platform

import (
	"os"
	"path/filepath"
)

// Follows the XDG base directory spec.
func defaultDir(home string) string {
	if d := os.Getenv("XDG_DATA_HOME"); d != "" {
		return filepath.Join(d, "claude-switch")
	}
	return filepath.Join(home, ".local", "share", "claude-switch")
}
