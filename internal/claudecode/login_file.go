//go:build !darwin

package claudecode

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/PuntoyComaTech/claude-switch/internal/app"
	"github.com/PuntoyComaTech/claude-switch/internal/fsutil"
	"github.com/PuntoyComaTech/claude-switch/internal/sysexec"
)

// On Linux and Windows Claude Code keeps the login in a 0600 file.
type fileLogin struct{ path string }

func newLoginStore(l Layout, _ sysexec.Runner) loginStore {
	return fileLogin{path: l.CredentialsFile()}
}

func (f fileLogin) read(context.Context) ([]byte, error) {
	b, err := os.ReadFile(f.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, app.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", f.path, err)
	}
	return b, nil
}

func (f fileLogin) write(_ context.Context, b []byte) error {
	return fsutil.WriteFileAtomic(f.path, b, 0o600)
}

func (fileLogin) cleanup(context.Context) {}
