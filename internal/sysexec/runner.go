// Package sysexec runs external commands behind an interface so adapters that
// shell out (security, claude, osascript) can be tested with fakes.
package sysexec

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Cmd describes one command run.
type Cmd struct {
	Name string
	Args []string
	// Env entries are added to the current environment; "KEY=" with an empty
	// value removes KEY.
	Env []string
	// Stdin is fed to the command. Ignored when Interactive.
	Stdin []byte
	// Interactive attaches the terminal (used for browser logins).
	Interactive bool
}

// Runner runs a command and returns its stdout.
type Runner interface {
	Run(ctx context.Context, c Cmd) ([]byte, error)
}

// ExitError carries a non-zero exit code and the command's stderr.
type ExitError struct {
	Code   int
	Stderr string
}

func (e *ExitError) Error() string {
	if e.Stderr != "" {
		return fmt.Sprintf("exit status %d: %s", e.Code, e.Stderr)
	}
	return fmt.Sprintf("exit status %d", e.Code)
}

// ExitCode returns the exit code inside err, or -1.
func ExitCode(err error) int {
	var e *ExitError
	if errors.As(err, &e) {
		return e.Code
	}
	return -1
}

// OS runs real processes.
type OS struct{}

// Run implements Runner.
func (OS) Run(ctx context.Context, c Cmd) ([]byte, error) {
	cmd := exec.CommandContext(ctx, c.Name, c.Args...)
	cmd.Env = mergeEnv(os.Environ(), c.Env)
	if c.Interactive {
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		return nil, wrap(cmd.Run(), "")
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdin = bytes.NewReader(c.Stdin)
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	return stdout.Bytes(), wrap(err, strings.TrimSpace(stderr.String()))
}

func wrap(err error, stderr string) error {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return &ExitError{Code: ee.ExitCode(), Stderr: stderr}
	}
	return err
}

func mergeEnv(base, extra []string) []string {
	out := make([]string, 0, len(base)+len(extra))
	override := map[string]bool{}
	for _, kv := range extra {
		k, _, _ := strings.Cut(kv, "=")
		override[k] = true
	}
	for _, kv := range base {
		k, _, _ := strings.Cut(kv, "=")
		if !override[k] {
			out = append(out, kv)
		}
	}
	for _, kv := range extra {
		if k, v, _ := strings.Cut(kv, "="); v != "" || !strings.HasSuffix(kv, "=") {
			out = append(out, k+"="+v)
		}
	}
	return out
}
