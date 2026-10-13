//go:build !darwin && !windows

package claudecode

// checkSupported: on Linux Claude Code always uses .credentials.json.
func checkSupported(Env, Layout) error { return nil }
