package claudecode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/PuntoyComaTech/claude-switch/internal/app"
	"github.com/PuntoyComaTech/claude-switch/internal/domain"
	"github.com/PuntoyComaTech/claude-switch/internal/fsutil"
	"github.com/PuntoyComaTech/claude-switch/internal/sysexec"
)

// loginStore reads and writes the raw login JSON of one configuration.
// Each platform provides one (login_darwin.go, login_other.go).
type loginStore interface {
	read(ctx context.Context) ([]byte, error) // app.ErrNotFound when absent
	write(ctx context.Context, b []byte) error
	// cleanup removes what a throwaway configuration left outside its dir.
	cleanup(ctx context.Context)
}

// Home implements app.Home for one Claude Code configuration.
type Home struct {
	layout Layout
	login  loginStore
	run    sysexec.Runner
	claude string
}

var _ app.Home = (*Home)(nil)

// ReadLogin implements app.Home.
func (h *Home) ReadLogin() (domain.Credentials, error) {
	b, err := h.login.read(context.Background())
	if err != nil {
		return nil, err
	}
	return domain.ParseCredentials(b)
}

// WriteLogin implements app.Home. Running sessions re-read the login when the
// credentials file mtime changes, so it is touched afterwards.
func (h *Home) WriteLogin(c domain.Credentials) error {
	b, err := c.Bytes()
	if err != nil {
		return err
	}
	if err := h.login.write(context.Background(), b); err != nil {
		return err
	}
	fsutil.Touch(h.layout.CredentialsFile())
	return nil
}

// ReadAccount implements app.Home.
func (h *Home) ReadAccount() (json.RawMessage, error) {
	cfg, err := readConfig(h.layout.ConfigFile)
	if err != nil {
		return nil, err
	}
	if v, ok := cfg["oauthAccount"]; ok {
		return v, nil
	}
	return json.RawMessage("null"), nil
}

// WriteAccount implements app.Home. It takes Claude Code's own config lock
// and keeps every other key of the file.
func (h *Home) WriteAccount(acct json.RawMessage) error {
	path := h.layout.ConfigFile
	unlock, err := fsutil.NewConfigLock(path).Lock(context.Background())
	if err != nil {
		return err
	}
	defer unlock()
	cfg, err := readConfig(path)
	if err != nil {
		return err
	}
	if len(acct) == 0 || string(acct) == "null" {
		delete(cfg, "oauthAccount")
	} else {
		cfg["oauthAccount"] = acct
	}
	b, err := domain.MarshalIndent(cfg)
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(path, b, fsutil.ModeOr(path, 0o600))
}

// AuthStatus implements app.Home.
func (h *Home) AuthStatus(ctx context.Context) (string, error) {
	out, err := h.run.Run(ctx, sysexec.Cmd{
		Name: h.claude, Args: []string{"auth", "status", "--json"}, Env: h.layout.RunEnv,
	})
	var st struct {
		LoggedIn bool    `json:"loggedIn"`
		Email    *string `json:"email"`
	}
	if jerr := json.Unmarshal(out, &st); jerr != nil {
		if err != nil {
			return "", fmt.Errorf("claude auth status: %w", err)
		}
		return "", fmt.Errorf("claude auth status: %w", jerr)
	}
	if st.Email == nil {
		return "", nil
	}
	return *st.Email, nil
}

// AuthLogin implements app.Home.
func (h *Home) AuthLogin(ctx context.Context, email string) error {
	args := []string{"auth", "login"}
	if email != "" {
		args = append(args, "--email", email)
	}
	_, err := h.run.Run(ctx, sysexec.Cmd{Name: h.claude, Args: args, Env: h.layout.RunEnv, Interactive: true})
	return err
}

func readConfig(path string) (map[string]json.RawMessage, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]json.RawMessage{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	cfg := map[string]json.RawMessage{}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return cfg, nil
}
