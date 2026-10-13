package fsutil

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"
)

// DirLock is the lock protocol Claude Code uses for its config file
// (proper-lockfile): the lock is a directory named <file>.lock, created with
// mkdir. A holder refreshes its mtime every Stale/2, so a lock whose mtime is
// older than Stale belongs to nobody and is broken.
//
// Claude Code leaves the directory behind in some exits (seen with
// `claude auth status`), so a waiter must be ready to wait out Stale.
type DirLock struct {
	Path  string
	Stale time.Duration
	// Wait is how long to keep trying; it should exceed Stale so a lock left
	// behind is always outlived.
	Wait       time.Duration
	MinBackoff time.Duration
	MaxBackoff time.Duration
}

// NewConfigLock returns the lock Claude Code takes for the config file at file.
func NewConfigLock(file string) DirLock {
	return DirLock{
		Path:       file + ".lock",
		Stale:      10 * time.Second,
		Wait:       12 * time.Second,
		MinBackoff: 20 * time.Millisecond,
		MaxBackoff: 320 * time.Millisecond,
	}
}

// ErrLocked means another process held the lock for the whole wait.
var ErrLocked = errors.New("locked by another process")

// Lock acquires the lock and returns its release function.
func (l DirLock) Lock(ctx context.Context) (func(), error) {
	deadline := time.Now().Add(l.Wait)
	wait := l.MinBackoff
	for {
		err := os.Mkdir(l.Path, 0o700)
		if err == nil {
			return func() { _ = os.Remove(l.Path) }, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("lock %s: %w", l.Path, err)
		}
		if l.isStale() {
			_ = os.Remove(l.Path)
			continue
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("%s: %w", l.Path, ErrLocked)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
		wait = min(wait*2, l.MaxBackoff)
	}
}

func (l DirLock) isStale() bool {
	fi, err := os.Stat(l.Path)
	return err == nil && time.Since(fi.ModTime()) > l.Stale
}
