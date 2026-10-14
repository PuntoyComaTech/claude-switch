// Package store persists claude-switch's own data: the account index
// (state.json), saved profiles, the process lock and the log.
package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"

	"github.com/PuntoyComaTech/claude-switch/internal/app"
	"github.com/PuntoyComaTech/claude-switch/internal/domain"
	"github.com/PuntoyComaTech/claude-switch/internal/fsutil"
)

// StateFile implements app.StateRepo with a JSON file.
type StateFile struct{ Path string }

var _ app.StateRepo = StateFile{}

// Load implements app.StateRepo. A missing file is an empty index.
func (f StateFile) Load() (*domain.State, error) {
	b, err := os.ReadFile(f.Path)
	if errors.Is(err, os.ErrNotExist) {
		return domain.NewState(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", f.Path, err)
	}
	st := &domain.State{}
	if err := json.Unmarshal(b, st); err != nil {
		return nil, fmt.Errorf("parse %s: %w", f.Path, err)
	}
	st.Normalize()
	return st, nil
}

// Save implements app.StateRepo.
func (f StateFile) Save(st *domain.State) error {
	b, err := domain.MarshalIndent(st)
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(f.Path, b, 0o600)
}

// FileLock implements app.Locker with an OS file lock, released by the
// kernel if the process dies.
type FileLock struct {
	Path    string
	Timeout time.Duration
}

var _ app.Locker = FileLock{}

// ErrBusy means another claude-switch held the lock for the whole timeout.
var ErrBusy = errors.New("another switch is in progress")

// Lock implements app.Locker.
func (l FileLock) Lock(ctx context.Context) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(l.Path), 0o700); err != nil {
		return nil, fmt.Errorf("create %s: %w", filepath.Dir(l.Path), err)
	}
	timeout := l.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	fl := flock.New(l.Path)
	ok, err := fl.TryLockContext(ctx, 100*time.Millisecond)
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		return nil, fmt.Errorf("lock %s: %w", l.Path, err)
	}
	if !ok {
		return nil, ErrBusy
	}
	return func() { _ = fl.Unlock() }, nil
}

// LogFile implements app.Logger by appending timestamped lines.
type LogFile struct{ Path string }

var _ app.Logger = LogFile{}

// Log implements app.Logger. Logging never fails the caller.
func (l LogFile) Log(msg string) {
	f, err := os.OpenFile(l.Path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()
	_, _ = fmt.Fprintf(f, "%s %s\n", time.Now().Format("2006-01-02 15:04:05"), msg)
}
