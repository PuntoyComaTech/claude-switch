package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/PuntoyComaTech/claude-switch/internal/app"
	"github.com/PuntoyComaTech/claude-switch/internal/domain"
	"github.com/PuntoyComaTech/claude-switch/internal/fsutil"
)

// FileProfiles keeps each profile in <Dir>/<name>.json with mode 0600. Used on
// Linux and Windows, where Claude Code itself keeps the login in a 0600 file.
type FileProfiles struct{ Dir string }

var _ app.ProfileStore = FileProfiles{}

func (f FileProfiles) path(name string) (string, error) {
	if err := domain.ValidateName(name); err != nil {
		return "", err
	}
	return filepath.Join(f.Dir, name+".json"), nil
}

// Load implements app.ProfileStore.
func (f FileProfiles) Load(name string) (domain.Profile, error) {
	p, err := f.path(name)
	if err != nil {
		return domain.Profile{}, err
	}
	b, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return domain.Profile{}, app.ErrNotFound
	}
	if err != nil {
		return domain.Profile{}, fmt.Errorf("read %s: %w", p, err)
	}
	return domain.ParseProfile(b)
}

// Save implements app.ProfileStore.
func (f FileProfiles) Save(name string, prof domain.Profile) error {
	p, err := f.path(name)
	if err != nil {
		return err
	}
	b, err := prof.Bytes()
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(p, b, 0o600)
}

// Delete implements app.ProfileStore.
func (f FileProfiles) Delete(name string) error {
	p, err := f.path(name)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("delete %s: %w", p, err)
	}
	return nil
}
