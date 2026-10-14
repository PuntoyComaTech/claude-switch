// Package fsutil has the file primitives every store shares: atomic writes
// and the mkdir lock Claude Code uses for its config file.
package fsutil

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// renameRetries covers Windows, where a rename fails while another process
// (Claude Code) briefly holds the target open.
const (
	renameRetries = 10
	renameBackoff = 50 * time.Millisecond
)

// WriteFileAtomic writes data to path through a temp file in the same
// directory, so readers never see a partial file. The final mode is perm.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*")
	if err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	name := tmp.Name()
	defer func() { _ = os.Remove(name) }() // no-op after a successful rename

	if err := writeAndClose(tmp, data, perm); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	for i := 0; ; i++ {
		err = os.Rename(name, path)
		if err == nil || i == renameRetries {
			break
		}
		time.Sleep(renameBackoff)
	}
	if err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return nil
}

func writeAndClose(f *os.File, data []byte, perm os.FileMode) error {
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Chmod(perm); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// ModeOr returns the mode of path, or fallback when it does not exist.
func ModeOr(path string, fallback os.FileMode) os.FileMode {
	if fi, err := os.Stat(path); err == nil {
		return fi.Mode().Perm()
	}
	return fallback
}

// Touch sets the modification time of path to now when the file exists.
func Touch(path string) {
	now := time.Now()
	_ = os.Chtimes(path, now, now)
}
