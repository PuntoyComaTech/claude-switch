package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/PuntoyComaTech/claude-switch/internal/app"
	"github.com/PuntoyComaTech/claude-switch/internal/domain"
	"github.com/PuntoyComaTech/claude-switch/internal/store"
)

// state.json written by the bash version must load unchanged.
const bashState = `{"active":"trabajo","order":["trabajo","personal"],
"accounts":{"trabajo":{"email":"w@x","rt_exp":null},"personal":{"email":"p@x","rt_exp":1799999999999}},
"usage":{"personal":{"five":2.0,"five_reset":1791937799,"seven":21.0,"seven_reset":1792216800,"at":1791920871.9,"src":"api"}},
"sessions":{"hermes-quota-watch":{"sig":"0.0|1791938399|71.0|1791939599","at":1791920871.6}},
"threshold":96,"last_switch":1791920771.0}`

func TestStateReadsBashFormat(t *testing.T) {
	p := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(p, []byte(bashState), 0o600); err != nil {
		t.Fatal(err)
	}
	f := store.StateFile{Path: p}
	st, err := f.Load()
	if err != nil {
		t.Fatal(err)
	}
	if st.ActiveName() != "trabajo" || st.EffectiveThreshold() != 96 || *st.Usage["personal"].Seven != 21 {
		t.Fatalf("decoded wrong: %+v", st)
	}
	if err := f.Save(st); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	var a, b2 any
	_ = json.Unmarshal([]byte(bashState), &a)
	_ = json.Unmarshal(b, &b2)
	ab, _ := json.Marshal(a)
	bb, _ := json.Marshal(b2)
	if string(ab) != string(bb) {
		t.Errorf("round trip changed state.json:\n%s\n%s", ab, bb)
	}
}

func TestMissingStateIsEmpty(t *testing.T) {
	st, err := store.StateFile{Path: filepath.Join(t.TempDir(), "none.json")}.Load()
	if err != nil || st.ActiveName() != "" || st.Accounts == nil {
		t.Fatalf("got %+v, %v", st, err)
	}
}

func TestFileProfiles(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "profiles")
	s := store.FileProfiles{Dir: dir}
	if _, err := s.Load("x"); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	creds, _ := domain.ParseCredentials([]byte(`{"claudeAiOauth":{"accessToken":"t"}}`))
	if err := s.Save("work", domain.NewProfile(creds, json.RawMessage(`{"emailAddress":"w@x"}`))); err != nil {
		t.Fatal(err)
	}
	p, err := s.Load("work")
	if err != nil || p.Email() != "w@x" || p.Login.AccessToken() != "t" {
		t.Fatalf("got %+v, %v", p, err)
	}
	if runtime.GOOS != "windows" {
		fi, _ := os.Stat(filepath.Join(dir, "work.json"))
		if fi.Mode().Perm() != 0o600 {
			t.Errorf("profile mode %v, want 0600", fi.Mode().Perm())
		}
	}
	if err := s.Save("../escape", domain.Profile{}); err == nil {
		t.Fatal("path traversal in a name must be rejected")
	}
	if err := s.Delete("work"); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete("work"); err != nil {
		t.Fatalf("deleting twice is not an error: %v", err)
	}
}

func TestFileLockExcludes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lock.flock")
	l := store.FileLock{Path: path, Timeout: 300 * time.Millisecond}
	unlock, err := l.Lock(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// flock is per open file description, so a second handle in the same
	// process contends like another process would.
	if _, err := l.Lock(context.Background()); !errors.Is(err, store.ErrBusy) {
		t.Fatalf("second lock: want ErrBusy, got %v", err)
	}
	unlock()
	unlock2, err := l.Lock(context.Background())
	if err != nil {
		t.Fatalf("after unlock: %v", err)
	}
	unlock2()
}
