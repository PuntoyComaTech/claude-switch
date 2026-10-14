package platform_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/PuntoyComaTech/claude-switch/internal/platform"
)

func TestDirOverride(t *testing.T) {
	t.Setenv("CLAUDE_SWITCH_DIR", "/x/cs")
	p := platform.ResolvePaths("/home/u")
	if p.Dir != "/x/cs" || p.State != filepath.Join("/x/cs", "state.json") {
		t.Errorf("got %+v", p)
	}
}

func TestDefaultDirPerOS(t *testing.T) {
	t.Setenv("CLAUDE_SWITCH_DIR", "")
	t.Setenv("XDG_DATA_HOME", "")
	home := filepath.Join(string(filepath.Separator), "home", "u")
	got := platform.ResolvePaths(home).Dir
	var want string
	switch runtime.GOOS {
	case "darwin":
		want = filepath.Join(home, ".claude-switch") // same as the bash version
	case "windows":
		t.Setenv("LOCALAPPDATA", `C:\Users\u\AppData\Local`)
		got = platform.ResolvePaths(home).Dir
		want = filepath.Join(`C:\Users\u\AppData\Local`, "claude-switch")
	default:
		want = filepath.Join(home, ".local", "share", "claude-switch")
	}
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
