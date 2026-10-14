package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/PuntoyComaTech/claude-switch/internal/domain"
)

var errNoAccount = errors.New("no oauthAccount in Claude Code's config; run 'claude' once after logging in")

// Deps are the adapters a Service needs.
type Deps struct {
	Profiles ProfileStore
	State    StateRepo
	Lock     Locker
	Homes    Homes
	Usage    UsageReader
	Notifier Notifier
	Logger   Logger
	// Out receives user-facing messages. Defaults to io.Discard.
	Out io.Writer
	// Now is the clock. Defaults to time.Now.
	Now func() time.Time
	// Threshold overrides the saved threshold when > 0 (CLAUDE_SWITCH_THRESHOLD).
	Threshold int
}

// Service implements every claude-switch command.
type Service struct {
	d Deps
}

// New builds a Service, filling optional dependencies with defaults.
func New(d Deps) *Service {
	if d.Out == nil {
		d.Out = io.Discard
	}
	if d.Now == nil {
		d.Now = time.Now
	}
	return &Service{d: d}
}

func (s *Service) now() time.Time { return s.d.Now() }

func (s *Service) say(format string, args ...any) {
	_, _ = fmt.Fprintf(s.d.Out, format+"\n", args...)
}

func (s *Service) log(format string, args ...any) { s.d.Logger.Log(fmt.Sprintf(format, args...)) }

func (s *Service) notify(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	s.log("notify: %s", msg)
	s.d.Notifier.Notify(msg)
}

// state loads the index without locking, for read-only decisions.
func (s *Service) state() (*domain.State, error) { return s.d.State.Load() }

// update runs fn on the index under the lock and saves it when fn succeeds.
func (s *Service) update(ctx context.Context, fn func(*domain.State) error) error {
	unlock, err := s.d.Lock.Lock(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	st, err := s.d.State.Load()
	if err != nil {
		return err
	}
	if err := fn(st); err != nil {
		return err
	}
	return s.d.State.Save(st)
}

func (s *Service) threshold(st *domain.State) int {
	if s.d.Threshold > 0 {
		return s.d.Threshold
	}
	return st.EffectiveThreshold()
}

// liveEmail returns the email of the account Claude Code is logged in with, or "".
func (s *Service) liveEmail() (string, error) {
	acct, err := s.d.Homes.Live().ReadAccount()
	if err != nil {
		return "", err
	}
	return domain.AccountEmail(acct), nil
}
