package claudecode

import (
	"encoding/json"
	"errors"
)

// errCredman is returned while Claude Code keeps the login in Windows
// Credential Manager, a backend claude-switch does not support yet.
var errCredman = errors.New("Claude Code is storing its login in Windows Credential Manager, " +
	"which claude-switch does not support yet. Please open an issue: " +
	"https://github.com/PuntoyComaTech/claude-switch/issues")

// checkSupported refuses to run when Claude Code's Credential Manager backend
// is on: CLAUDE_CODE_FORCE_WINDOWS_CREDMAN=1, or the tengu_windows_credman
// feature cached in the config file. Otherwise Claude Code uses the plaintext
// .credentials.json file, which this package handles.
func checkSupported(env Env, l Layout) error {
	if v, _ := env("CLAUDE_CODE_FORCE_WINDOWS_CREDMAN"); v == "1" {
		return errCredman
	}
	cfg, err := readConfig(l.ConfigFile)
	if err != nil {
		return nil //nolint:nilerr // an unreadable config cannot enable the feature
	}
	var f struct {
		Credman bool `json:"tengu_windows_credman"`
	}
	if raw, ok := cfg["cachedGrowthBookFeatures"]; ok && json.Unmarshal(raw, &f) == nil && f.Credman {
		return errCredman
	}
	return nil
}
