package fsutil_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/PuntoyComaTech/claude-switch/internal/fsutil"
)

func testLock(t *testing.T) fsutil.DirLock {
	t.Helper()
	return fsutil.DirLock{
		Path:       filepath.Join(t.TempDir(), "cfg.json.lock"),
		Stale:      400 * time.Millisecond,
		Wait:       600 * time.Millisecond,
		MinBackoff: 10 * time.Millisecond,
		MaxBackoff: 40 * time.Millisecond,
	}
}

func TestDirLockAcquireRelease(t *testing.T) {
	l := testLock(t)
	unlock, err := l.Lock(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	unlock()
	if _, err := os.Stat(l.Path); !os.IsNotExist(err) {
		t.Fatal("release must remove the lock dir")
	}
}

// A lock Claude Code left behind must be outlived, not given up on.
func TestDirLockOutlivesAbandonedLock(t *testing.T) {
	l := testLock(t)
	if err := os.Mkdir(l.Path, 0o700); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	unlock, err := l.Lock(context.Background())
	if err != nil {
		t.Fatalf("an abandoned lock must be broken once stale: %v", err)
	}
	unlock()
	if time.Since(start) < l.Stale {
		t.Fatal("a fresh lock must not be broken early")
	}
}

func TestDirLockWaitsForRelease(t *testing.T) {
	l := testLock(t)
	if err := os.Mkdir(l.Path, 0o700); err != nil {
		t.Fatal(err)
	}
	go func() {
		time.Sleep(100 * time.Millisecond)
		_ = os.Remove(l.Path)
	}()
	unlock, err := l.Lock(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	unlock()
}

// A live holder keeps the mtime fresh; the waiter gives up after Wait.
func TestDirLockGivesUpOnLiveHolder(t *testing.T) {
	l := testLock(t)
	if err := os.Mkdir(l.Path, 0o700); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	defer close(done)
	go func() {
		for {
			select {
			case <-done:
				return
			case <-time.After(50 * time.Millisecond):
				now := time.Now()
				_ = os.Chtimes(l.Path, now, now)
			}
		}
	}()
	if _, err := l.Lock(context.Background()); !errors.Is(err, fsutil.ErrLocked) {
		t.Fatalf("want ErrLocked, got %v", err)
	}
}

func TestWriteFileAtomic(t *testing.T) {
	p := filepath.Join(t.TempDir(), "sub", "f.json")
	if err := fsutil.WriteFileAtomic(p, []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := fsutil.WriteFileAtomic(p, []byte("two"), 0o600); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	if string(b) != "two" {
		t.Fatalf("got %q", b)
	}
	entries, _ := os.ReadDir(filepath.Dir(p))
	if len(entries) != 1 {
		t.Fatalf("temp files left behind: %v", entries)
	}
}
