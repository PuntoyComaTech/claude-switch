package claudecode

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/PuntoyComaTech/claude-switch/internal/sysexec"
)

type stubRunner struct{ out []byte }

func (s stubRunner) Run(context.Context, sysexec.Cmd) ([]byte, error) { return s.out, nil }

func testHome(t *testing.T) (*Home, Layout) {
	t.Helper()
	dir := t.TempDir()
	l := Layout{ConfigDir: dir, SecureDir: dir, ConfigFile: filepath.Join(dir, ".claude.json")}
	return &Home{layout: l, login: memLogin{}, run: stubRunner{}, claude: "claude"}, l
}

type memLogin struct{}

func (memLogin) read(context.Context) ([]byte, error) { return []byte(`{}`), nil }
func (memLogin) write(context.Context, []byte) error  { return nil }
func (memLogin) cleanup(context.Context)              {}

func TestWriteAccountKeepsOtherKeys(t *testing.T) {
	h, l := testHome(t)
	orig := `{"numStartups":3,"projects":{"/a":{"x":1}},"oauthAccount":{"emailAddress":"old@x"}}`
	if err := os.WriteFile(l.ConfigFile, []byte(orig), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := h.WriteAccount(json.RawMessage(`{"emailAddress":"new@x"}`)); err != nil {
		t.Fatal(err)
	}
	var got map[string]json.RawMessage
	b, _ := os.ReadFile(l.ConfigFile)
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if string(got["numStartups"]) != "3" || !strings.Contains(string(got["projects"]), `"x": 1`) {
		t.Errorf("other keys changed: %s", b)
	}
	acct, _ := h.ReadAccount()
	if !strings.Contains(string(acct), "new@x") {
		t.Errorf("account not written: %s", acct)
	}
	// Windows has no Unix permission bits.
	if fi, _ := os.Stat(l.ConfigFile); runtime.GOOS != "windows" && fi.Mode().Perm() != 0o640 {
		t.Errorf("mode changed to %v", fi.Mode().Perm())
	}
	if _, err := os.Stat(l.ConfigFile + ".lock"); !os.IsNotExist(err) {
		t.Error("config lock left behind")
	}

	if err := h.WriteAccount(json.RawMessage("null")); err != nil {
		t.Fatal(err)
	}
	if acct, _ := h.ReadAccount(); string(acct) != "null" {
		t.Errorf("null must remove oauthAccount, got %s", acct)
	}
}

// Timing of a fresh lock is covered in fsutil; here: a stale lock left by
// Claude Code is broken and the write goes through.
func TestWriteAccountBreaksStaleClaudeCodeLock(t *testing.T) {
	h, l := testHome(t)
	lock := l.ConfigFile + ".lock"
	if err := os.Mkdir(lock, 0o700); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-11 * time.Second)
	if err := os.Chtimes(lock, old, old); err != nil {
		t.Fatal(err)
	}
	if err := h.WriteAccount(json.RawMessage(`{}`)); err != nil {
		t.Fatalf("a stale lock must be broken: %v", err)
	}
}

func TestWriteLoginTouchesCredentialsFile(t *testing.T) {
	h, l := testHome(t)
	f := l.CredentialsFile()
	if err := os.WriteFile(f, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	_ = os.Chtimes(f, old, old)
	if err := h.WriteLogin(nil); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Stat(f); time.Since(fi.ModTime()) > time.Minute {
		t.Error("running sessions watch this mtime; it must be bumped")
	}
}

// Claude Code reports email:null when the config has no oauthAccount.
func TestAuthStatusNullEmail(t *testing.T) {
	h, _ := testHome(t)
	h.run = stubRunner{out: []byte(`{"loggedIn":true,"email":null}`)}
	if email, err := h.AuthStatus(context.Background()); err != nil || email != "" {
		t.Fatalf("got %q, %v", email, err)
	}
	h.run = stubRunner{out: []byte(`{"loggedIn":true,"email":"a@x"}`)}
	if email, _ := h.AuthStatus(context.Background()); email != "a@x" {
		t.Fatalf("got %q", email)
	}
}
