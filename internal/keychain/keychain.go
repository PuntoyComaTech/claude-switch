// Package keychain reads and writes macOS Keychain generic passwords through
// /usr/bin/security, the same way Claude Code does.
//
// Items are never created through the Security framework: an item created by
// another binary makes Claude Code's own `security` reads show a password
// prompt. Secrets go to `security -i` on stdin as hex, so they never appear in
// the process list, except for payloads too large for one stdin line, where
// Claude Code itself falls back to argv.
package keychain

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/PuntoyComaTech/claude-switch/internal/sysexec"
)

const (
	// maxStdinLine is the longest `security -i` line Claude Code sends; longer
	// commands are rejected by security, so it falls back to argv.
	maxStdinLine = 4032
	// exitNotFound is security's exit code for a missing item.
	exitNotFound = 44
	timeout      = 10 * time.Second
	securityBin  = "security"
)

// ErrNotFound means the item does not exist.
var ErrNotFound = errors.New("keychain item not found")

// Keychain talks to the login keychain.
type Keychain struct {
	Run sysexec.Runner
}

// Get returns the secret of service/account.
func (k Keychain) Get(ctx context.Context, service, account string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	out, err := k.Run.Run(ctx, sysexec.Cmd{
		Name: securityBin,
		Args: []string{"find-generic-password", "-s", service, "-a", account, "-w"},
	})
	if sysexec.ExitCode(err) == exitNotFound {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read keychain item %s/%s: %w", service, account, err)
	}
	return bytes.TrimRight(out, "\n"), nil
}

// Set creates or replaces service/account and verifies it by reading it back.
func (k Keychain) Set(ctx context.Context, service, account string, secret []byte) error {
	if strings.ContainsAny(service+account, "\"\\\n") {
		return fmt.Errorf("invalid keychain name %q/%q", service, account)
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	h := hex.EncodeToString(secret)
	line := fmt.Sprintf("add-generic-password -U -a %q -s %q -X %q\n", account, service, h)
	cmd := sysexec.Cmd{Name: securityBin, Args: []string{"-i"}, Stdin: []byte(line)}
	if len(line) > maxStdinLine {
		cmd = sysexec.Cmd{Name: securityBin, Args: []string{"add-generic-password", "-U", "-a", account, "-s", service, "-X", h}}
	}
	if _, err := k.Run.Run(ctx, cmd); err != nil {
		return fmt.Errorf("write keychain item %s/%s: %w", service, account, err)
	}
	got, err := k.Get(ctx, service, account)
	if err != nil {
		return err
	}
	if !bytes.Equal(got, secret) {
		return fmt.Errorf("keychain write to %s/%s did not verify", service, account)
	}
	return nil
}

// Delete removes service/account. A missing item is not an error.
func (k Keychain) Delete(ctx context.Context, service, account string) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	_, err := k.Run.Run(ctx, sysexec.Cmd{
		Name: securityBin,
		Args: []string{"delete-generic-password", "-s", service, "-a", account},
	})
	if err != nil && sysexec.ExitCode(err) != exitNotFound {
		return fmt.Errorf("delete keychain item %s/%s: %w", service, account, err)
	}
	return nil
}
