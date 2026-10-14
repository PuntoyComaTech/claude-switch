package claudecode

import (
	"os"
	"path/filepath"
	"testing"
)

func envOf(m map[string]string) Env {
	return func(k string) (string, bool) {
		v, ok := m[k]
		return v, ok
	}
}

// Expected suffixes were computed by Claude Code 2.1.296's own rule
// (sha256 of the dir, first 8 hex chars) during the Linux experiment.
func TestKeychainServiceMatchesClaudeCode(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want string
	}{
		{"default", nil, "Claude Code-credentials"},
		{"config dir", map[string]string{"CLAUDE_CONFIG_DIR": "/tmp/e2/cfgA"}, "Claude Code-credentials-c2d9cc5e"},
		{"secure storage dir", map[string]string{"CLAUDE_CONFIG_DIR": "/tmp/e6/cfg", "CLAUDE_SECURESTORAGE_CONFIG_DIR": "/tmp/e6/sec"}, "Claude Code-credentials-33b1b6b8"},
		{"empty secure dir means default", map[string]string{"CLAUDE_SECURESTORAGE_CONFIG_DIR": ""}, "Claude Code-credentials"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := LiveLayout(envOf(c.env), "/home/u", "u").KeychainService
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestLayoutPaths(t *testing.T) {
	home := t.TempDir()
	l := LiveLayout(envOf(nil), home, "u")
	if l.ConfigFile != filepath.Join(home, ".claude.json") {
		t.Errorf("default config file: %s", l.ConfigFile)
	}
	if l.CredentialsFile() != filepath.Join(home, ".claude", ".credentials.json") {
		t.Errorf("default credentials file: %s", l.CredentialsFile())
	}

	cfg := filepath.Join(home, "cfg")
	sec := filepath.Join(home, "sec")
	l = LiveLayout(envOf(map[string]string{"CLAUDE_CONFIG_DIR": cfg, "CLAUDE_SECURESTORAGE_CONFIG_DIR": sec}), home, "u")
	if l.ConfigFile != filepath.Join(cfg, ".claude.json") || l.SecureDir != sec {
		t.Errorf("custom dirs: %+v", l)
	}
}

// Claude Code reads a legacy .config.json in the config dir before .claude.json.
func TestLegacyConfigWins(t *testing.T) {
	home := t.TempDir()
	cfg := filepath.Join(home, "cfg")
	if err := os.MkdirAll(cfg, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg, ".config.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	l := LiveLayout(envOf(map[string]string{"CLAUDE_CONFIG_DIR": cfg}), home, "u")
	if filepath.Base(l.ConfigFile) != ".config.json" {
		t.Errorf("got %s", l.ConfigFile)
	}
}

func TestKeychainAccountFallsBackLikeClaudeCode(t *testing.T) {
	if got := keychainAccount("gabriel"); got != "gabriel" {
		t.Errorf("got %q", got)
	}
	if got := keychainAccount("DOMAIN\\name with space"); got != "claude-code-user" {
		t.Errorf("got %q", got)
	}
}

func TestTempLayoutIsIsolated(t *testing.T) {
	l := TempLayout("/tmp/x", "u")
	if l.KeychainService == keychainBase {
		t.Error("a throwaway config must not use the live keychain item")
	}
	want := map[string]bool{"CLAUDE_CONFIG_DIR=/tmp/x": true, "CLAUDE_SECURESTORAGE_CONFIG_DIR=": true}
	for _, e := range l.RunEnv {
		delete(want, e)
	}
	if len(want) != 0 {
		t.Errorf("missing env: %v", want)
	}
}
