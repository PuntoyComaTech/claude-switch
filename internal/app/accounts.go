package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/PuntoyComaTech/claude-switch/internal/domain"
)

// Add saves the account Claude Code is logged in with as name and marks it active.
func (s *Service) Add(ctx context.Context, name string) error {
	if err := domain.ValidateName(name); err != nil {
		return err
	}
	email, err := s.liveEmail()
	if err != nil {
		return err
	}
	if email == "" {
		return errNoAccount
	}
	err = s.update(ctx, func(st *domain.State) error {
		if st.Has(name) && st.Accounts[name].Email != email {
			return fmt.Errorf("'%s' is '%s' but you are logged in as '%s'", name, st.Accounts[name].Email, email)
		}
		if other := st.NameForEmail(email); other != "" && other != name {
			return fmt.Errorf("'%s' is already saved as '%s'", email, other)
		}
		if err := s.snapshot(st, name); err != nil {
			return err
		}
		st.Active = &name
		return nil
	})
	if err != nil {
		return err
	}
	s.say("saved '%s' (%s) and marked it active", name, email)
	return nil
}

// AddWithLogin logs in to another account in a throwaway config and saves it,
// without touching the live login or running sessions.
func (s *Service) AddWithLogin(ctx context.Context, name string) error {
	if err := domain.ValidateName(name); err != nil {
		return err
	}
	st, err := s.state()
	if err != nil {
		return err
	}
	if st.Has(name) {
		return fmt.Errorf("'%s' already exists (%s)", name, st.Accounts[name].Email)
	}
	return s.isolatedLogin(ctx, name, "")
}

// Use switches Claude Code to a saved account. With auto, a dead login is
// reported with a notification instead of opening a browser.
func (s *Service) Use(ctx context.Context, name string, auto bool) error {
	st, err := s.state()
	if err != nil {
		return err
	}
	if err := st.Require(name); err != nil {
		return err
	}
	if st.ActiveName() == name {
		s.say("already on '%s'", name)
		return nil
	}
	if st.IsDead(name, s.now()) {
		if auto {
			s.notify("'%s' needs a new login: claude-switch login %s", name, name)
			return fmt.Errorf("'%s' needs a new login", name)
		}
		s.say("'%s' refresh token expired, logging in again", name)
		if err := s.Login(ctx, name); err != nil {
			return err
		}
	}
	err = s.update(ctx, func(st *domain.State) error {
		if err := s.snapshotActive(st); err != nil {
			return err
		}
		return s.restore(st, name)
	})
	if err != nil {
		return err
	}
	s.log("switched to %s", name)
	s.say("now on '%s' (%s)", name, st.Accounts[name].Email)
	return nil
}

// Next switches to the next account that is not at its limit.
func (s *Service) Next(ctx context.Context, auto bool) error {
	st, err := s.state()
	if err != nil {
		return err
	}
	now, limit := s.now(), s.threshold(st)
	for _, name := range st.Candidates() {
		if st.IsExhausted(name, limit, now) {
			continue
		}
		if auto && st.IsDead(name, now) {
			s.notify("'%s' needs a new login: claude-switch login %s", name, name)
			continue
		}
		err := s.Use(ctx, name, auto)
		if err == nil {
			return nil
		}
		if !auto {
			return err
		}
		s.log("switch to %s failed: %v", name, err)
	}
	if auto {
		s.notify("every account is at its limit; staying on '%s'", st.ActiveName())
	}
	return errors.New("no other account available")
}

// Login logs in again to a saved account, in a throwaway config. When it is
// the account in use, the live login is refreshed too.
func (s *Service) Login(ctx context.Context, name string) error {
	st, err := s.state()
	if err != nil {
		return err
	}
	if err := st.Require(name); err != nil {
		return err
	}
	email := st.Accounts[name].Email
	if err := s.isolatedLogin(ctx, name, email); err != nil {
		return err
	}
	live, err := s.liveEmail()
	if err != nil || live != email {
		return err
	}
	return s.update(ctx, func(st *domain.State) error { return s.restore(st, name) })
}

// Rename renames a saved account.
func (s *Service) Rename(ctx context.Context, oldName, newName string) error {
	if err := domain.ValidateName(newName); err != nil {
		return err
	}
	err := s.update(ctx, func(st *domain.State) error {
		if err := st.Rename(oldName, newName); err != nil {
			return err
		}
		p, err := s.d.Profiles.Load(oldName)
		if err != nil {
			return fmt.Errorf("no saved account '%s': %w", oldName, err)
		}
		if err := s.d.Profiles.Save(newName, p); err != nil {
			return err
		}
		return s.d.Profiles.Delete(oldName)
	})
	if err != nil {
		return err
	}
	s.say("renamed '%s' to '%s'", oldName, newName)
	return nil
}

// Remove deletes a saved account that is not active.
func (s *Service) Remove(ctx context.Context, name string) error {
	err := s.update(ctx, func(st *domain.State) error {
		if err := st.Remove(name); err != nil {
			return err
		}
		if err := s.d.Profiles.Delete(name); err != nil && !errors.Is(err, ErrNotFound) {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	s.say("removed '%s'", name)
	return nil
}

// ActiveName returns the active account name, or "".
func (s *Service) ActiveName() (string, error) {
	st, err := s.state()
	if err != nil {
		return "", err
	}
	return st.ActiveName(), nil
}

// Current prints the active account name.
func (s *Service) Current() error {
	n, err := s.ActiveName()
	if err == nil && n != "" {
		s.say("%s", n)
	}
	return err
}

// Threshold prints the threshold, or saves a new one when value is not empty.
func (s *Service) Threshold(ctx context.Context, value string) error {
	if value == "" {
		st, err := s.state()
		if err != nil {
			return err
		}
		s.say("%d", s.threshold(st))
		return nil
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return errors.New("threshold must be 1-100")
	}
	if err := s.update(ctx, func(st *domain.State) error { return st.SetThreshold(n) }); err != nil {
		return err
	}
	s.say("switching at %d%%", n)
	return nil
}

// --- shared steps (callers hold the lock) -----------------------------------

func (s *Service) saveProfile(st *domain.State, name string, p domain.Profile, email string) error {
	if err := s.d.Profiles.Save(name, p); err != nil {
		return err
	}
	st.Upsert(name, domain.Account{Email: email, RefreshExpiry: p.Login.RefreshTokenExpiry()})
	return nil
}

// snapshot copies the live login into profile name.
func (s *Service) snapshot(st *domain.State, name string) error {
	live := s.d.Homes.Live()
	creds, err := live.ReadLogin()
	if errors.Is(err, ErrNotFound) {
		return errors.New("no Claude Code login found")
	}
	if err != nil {
		return err
	}
	acct, err := live.ReadAccount()
	if err != nil {
		return err
	}
	email := domain.AccountEmail(acct)
	if email == "" {
		email = st.Accounts[name].Email
	}
	if email == "" {
		return errNoAccount
	}
	return s.saveProfile(st, name, domain.NewProfile(creds, acct), email)
}

// snapshotActive saves the live login before it is replaced, since Claude
// Code rotates its tokens while it runs.
func (s *Service) snapshotActive(st *domain.State) error {
	email, err := s.liveEmail()
	if err != nil {
		return err
	}
	name := st.ActiveName()
	if email != "" {
		name = st.NameForEmail(email)
		if name == "" {
			return fmt.Errorf("you are logged in as '%s', which is not saved. Save it first: claude-switch add <name>", email)
		}
	}
	if name == "" {
		return nil
	}
	return s.snapshot(st, name)
}

// restore puts profile name into the live login, keeping the machine's
// non-login keys.
func (s *Service) restore(st *domain.State, name string) error {
	p, err := s.d.Profiles.Load(name)
	if err != nil {
		return fmt.Errorf("no saved account '%s': %w", name, err)
	}
	live := s.d.Homes.Live()
	cur, err := live.ReadLogin()
	if errors.Is(err, ErrNotFound) {
		cur = domain.Credentials{}
	} else if err != nil {
		return err
	}
	if err := live.WriteLogin(cur.WithLogin(p.Login)); err != nil {
		return err
	}
	if err := live.WriteAccount(p.OAuthAccount); err != nil {
		return err
	}
	st.SetActive(name, s.now())
	return nil
}

// isolatedLogin logs in inside a throwaway config and saves the result as
// name. expected pins the email when not empty.
func (s *Service) isolatedLogin(ctx context.Context, name, expected string) error {
	live := s.d.Homes.Live()
	before, beforeErr := live.ReadLogin()

	tmp, cleanup, err := s.d.Homes.Temp()
	if err != nil {
		return err
	}
	defer cleanup()
	if err := tmp.AuthLogin(ctx, expected); err != nil {
		return fmt.Errorf("login failed: %w", err)
	}
	creds, err := tmp.ReadLogin()
	if err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}
	if beforeErr == nil {
		if creds, err = s.guardLiveLogin(live, before, creds); err != nil {
			return err
		}
	}
	if len(creds.LoginOnly()) == 0 {
		return errors.New("login did not produce credentials")
	}
	acct, email, err := loggedInAs(ctx, tmp, expected)
	if err != nil {
		return err
	}
	err = s.update(ctx, func(st *domain.State) error {
		if other := st.NameForEmail(email); other != "" && other != name {
			return fmt.Errorf("'%s' is already saved as '%s'", email, other)
		}
		return s.saveProfile(st, name, domain.NewProfile(creds, acct), email)
	})
	if err != nil {
		return err
	}
	s.say("saved '%s' (%s)", name, email)
	return nil
}

// loggedInAs tells which account a fresh login in h belongs to, and checks it
// against expected when not empty.
func loggedInAs(ctx context.Context, h Home, expected string) (json.RawMessage, string, error) {
	acct, err := h.ReadAccount()
	if err != nil {
		return nil, "", err
	}
	email := domain.AccountEmail(acct)
	if email == "" {
		email, _ = h.AuthStatus(ctx)
	}
	if email == "" {
		return nil, "", errors.New("could not tell which account logged in")
	}
	if expected != "" && email != expected {
		return nil, "", fmt.Errorf("logged in as '%s', expected '%s'", email, expected)
	}
	return acct, email, nil
}

// guardLiveLogin is a safety net for Claude Code versions that ignore the
// throwaway config dir and write the live login instead: it puts the live
// login back and returns what the login wrote as the new credentials.
func (s *Service) guardLiveLogin(live Home, before, creds domain.Credentials) (domain.Credentials, error) {
	after, err := live.ReadLogin()
	if err != nil || sameCredentials(before, after) {
		return creds, nil //nolint:nilerr // an unreadable live login means nothing to restore
	}
	if creds == nil {
		creds = after
	}
	if err := live.WriteLogin(before); err != nil {
		return nil, err
	}
	s.log("isolated login touched the live login; restored")
	s.say("claude-switch: login touched the live login; restored it")
	return creds, nil
}

func sameCredentials(a, b domain.Credentials) bool {
	x, errA := a.Bytes()
	y, errB := b.Bytes()
	return errA == nil && errB == nil && bytes.Equal(x, y)
}
