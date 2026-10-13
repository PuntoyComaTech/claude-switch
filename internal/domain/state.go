package domain

import (
	"errors"
	"fmt"
	"slices"
	"time"
)

// DefaultThreshold is the usage percent that triggers a switch when none is saved.
const DefaultThreshold = 95

// sessionTTL is how long a status-line session signature is remembered.
const sessionTTL = 24 * time.Hour

// ErrUnknownAccount is returned for a name that is not saved.
var ErrUnknownAccount = errors.New("unknown account")

// State is claude-switch's index of saved accounts and their usage. Its JSON
// shape is the bash version's state.json, so both can share one directory.
type State struct {
	Active     *string            `json:"active"`
	Order      []string           `json:"order"`
	Accounts   map[string]Account `json:"accounts"`
	Usage      map[string]Usage   `json:"usage"`
	Sessions   map[string]Session `json:"sessions"`
	Threshold  *int               `json:"threshold,omitempty"`
	LastSwitch *float64           `json:"last_switch,omitempty"`
}

// Account is what the index knows about a saved account.
type Account struct {
	Email string `json:"email"`
	// RefreshExpiry is the refresh token expiry in Unix milliseconds, if known.
	RefreshExpiry *float64 `json:"rt_exp"`
}

// Usage is the last known usage of an account. Percentages are 0-100 and
// reset times are Unix seconds. A nil field means the window is unknown.
type Usage struct {
	Five       *float64 `json:"five"`
	FiveReset  *float64 `json:"five_reset"`
	Seven      *float64 `json:"seven"`
	SevenReset *float64 `json:"seven_reset"`
	At         float64  `json:"at"`
	Source     string   `json:"src,omitempty"`
}

// Session remembers the last usage signature a status-line session reported.
type Session struct {
	Sig string  `json:"sig"`
	At  float64 `json:"at"`
}

// NewState returns an empty index.
func NewState() *State {
	s := &State{}
	s.normalize()
	return s
}

// normalize fills nil collections so callers never check for them.
func (s *State) normalize() {
	if s.Order == nil {
		s.Order = []string{}
	}
	if s.Accounts == nil {
		s.Accounts = map[string]Account{}
	}
	if s.Usage == nil {
		s.Usage = map[string]Usage{}
	}
	if s.Sessions == nil {
		s.Sessions = map[string]Session{}
	}
}

// Normalize prepares a decoded state for use.
func (s *State) Normalize() { s.normalize() }

// ActiveName returns the active account, or "".
func (s *State) ActiveName() string {
	if s.Active == nil {
		return ""
	}
	return *s.Active
}

// SetActive marks name active.
func (s *State) SetActive(name string, now time.Time) {
	s.Active = &name
	t := unixSeconds(now)
	s.LastSwitch = &t
}

// Has reports whether name is saved.
func (s *State) Has(name string) bool {
	_, ok := s.Accounts[name]
	return ok
}

// Require returns ErrUnknownAccount when name is not saved.
func (s *State) Require(name string) error {
	if !s.Has(name) {
		return fmt.Errorf("%w %q. See: claude-switch list", ErrUnknownAccount, name)
	}
	return nil
}

// NameForEmail returns the saved account using email, or "".
func (s *State) NameForEmail(email string) string {
	for _, n := range s.Order {
		if s.Accounts[n].Email == email {
			return n
		}
	}
	return ""
}

// Upsert records an account, appending it to the order when new.
func (s *State) Upsert(name string, acct Account) {
	s.Accounts[name] = acct
	if !slices.Contains(s.Order, name) {
		s.Order = append(s.Order, name)
	}
}

// Rename moves an account, its usage and its place in the order.
func (s *State) Rename(oldName, newName string) error {
	if err := s.Require(oldName); err != nil {
		return err
	}
	if s.Has(newName) {
		return fmt.Errorf("%q already exists", newName)
	}
	s.Accounts[newName] = s.Accounts[oldName]
	delete(s.Accounts, oldName)
	if u, ok := s.Usage[oldName]; ok {
		s.Usage[newName] = u
		delete(s.Usage, oldName)
	}
	for i, n := range s.Order {
		if n == oldName {
			s.Order[i] = newName
		}
	}
	if s.ActiveName() == oldName {
		s.Active = &newName
	}
	return nil
}

// Remove deletes an inactive account.
func (s *State) Remove(name string) error {
	if err := s.Require(name); err != nil {
		return err
	}
	if s.ActiveName() == name {
		return fmt.Errorf("%q is active; switch first", name)
	}
	delete(s.Accounts, name)
	delete(s.Usage, name)
	s.Order = slices.DeleteFunc(s.Order, func(n string) bool { return n == name })
	return nil
}

// IsDead reports whether the account's refresh token is known to have expired.
func (s *State) IsDead(name string, now time.Time) bool {
	rt := s.Accounts[name].RefreshExpiry
	return rt != nil && time.UnixMilli(int64(*rt)).Before(now)
}

// IsExhausted reports whether a window of the account is at or over the
// threshold and has not reset yet.
func (s *State) IsExhausted(name string, threshold int, now time.Time) bool {
	u, ok := s.Usage[name]
	if !ok {
		return false
	}
	t := float64(threshold)
	n := unixSeconds(now)
	over := func(p, reset *float64) bool { return p != nil && *p >= t && reset != nil && *reset > n }
	return over(u.Five, u.FiveReset) || over(u.Seven, u.SevenReset)
}

// Candidates returns the accounts after the active one, wrapping around,
// without the active one.
func (s *State) Candidates() []string {
	active := s.ActiveName()
	i := slices.Index(s.Order, active)
	rotated := append(slices.Clone(s.Order[i+1:]), s.Order[:i+1]...)
	return slices.DeleteFunc(rotated, func(n string) bool { return n == active })
}

// SetUsage stores usage read for an account.
func (s *State) SetUsage(name string, u Usage) { s.Usage[name] = u }

// EffectiveThreshold returns the saved threshold or the default.
func (s *State) EffectiveThreshold() int {
	if s.Threshold != nil {
		return *s.Threshold
	}
	return DefaultThreshold
}

// SetThreshold validates and saves a threshold.
func (s *State) SetThreshold(n int) error {
	if n < 1 || n > 100 {
		return errors.New("threshold must be 1-100")
	}
	s.Threshold = &n
	return nil
}

func unixSeconds(t time.Time) float64 { return float64(t.UnixNano()) / 1e9 }
