// Package app implements claude-switch's use cases on top of small ports.
// Adapters for each operating system live in other packages; this package
// only sees the interfaces below, so every use case is testable with fakes.
package app

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/PuntoyComaTech/claude-switch/internal/domain"
)

// ErrNotFound is returned by stores when an item does not exist.
var ErrNotFound = errors.New("not found")

// ProfileStore keeps claude-switch's saved accounts.
type ProfileStore interface {
	Load(name string) (domain.Profile, error)
	Save(name string, p domain.Profile) error
	Delete(name string) error
}

// StateRepo loads and saves the account index. Callers hold the Locker.
type StateRepo interface {
	Load() (*domain.State, error)
	Save(*domain.State) error
}

// Locker serialises claude-switch processes.
type Locker interface {
	Lock(ctx context.Context) (unlock func(), err error)
}

// Home is one Claude Code configuration: the user's live one or a throwaway
// one used to log in or refresh a token without touching running sessions.
type Home interface {
	// ReadLogin returns the stored credentials, or ErrNotFound.
	ReadLogin() (domain.Credentials, error)
	WriteLogin(domain.Credentials) error
	// ReadAccount returns the oauthAccount block, or JSON null.
	ReadAccount() (json.RawMessage, error)
	// WriteAccount replaces the oauthAccount block; JSON null removes it.
	WriteAccount(json.RawMessage) error
	// AuthStatus runs `claude auth status` here. Claude Code refreshes an
	// expired access token as a side effect. It returns the reported email.
	AuthStatus(ctx context.Context) (email string, err error)
	// AuthLogin runs the browser login here, pinned to email when not empty.
	AuthLogin(ctx context.Context, email string) error
}

// Homes gives access to the live configuration and creates throwaway ones.
type Homes interface {
	Live() Home
	// Temp creates a throwaway configuration. cleanup removes every trace.
	Temp() (h Home, cleanup func(), err error)
}

// UsageReader reads an account's usage with its access token.
type UsageReader interface {
	Read(ctx context.Context, accessToken string) (domain.Usage, error)
}

// Notifier shows a desktop notification. Failures are ignored by callers.
type Notifier interface {
	Notify(msg string)
}

// Logger appends a line to claude-switch's log.
type Logger interface {
	Log(msg string)
}
