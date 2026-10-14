package app

import (
	"context"
	"errors"
	"sync"

	"github.com/PuntoyComaTech/claude-switch/internal/domain"
)

// Report records the status-line usage of the active account and switches
// to the next account when a window crosses the threshold.
func (s *Service) Report(ctx context.Context, in domain.StatusLine) error {
	var shouldSwitch bool
	var active string
	err := s.update(ctx, func(st *domain.State) error {
		active = st.ActiveName()
		shouldSwitch = st.RecordReport(in, s.threshold(st), s.now())
		return nil
	})
	if err != nil || !shouldSwitch {
		return err
	}
	s.log("limit reached on %s", active)
	return s.Next(ctx, true)
}

// AccountView is one row of `list`.
type AccountView struct {
	Name   string
	Email  string
	Active bool
	Dead   bool
	// Usage is nil when nothing is known. Windows whose reset time has passed
	// are cleared by the caller's renderer via Usage reset times.
	Usage *domain.Usage
}

// ListView is everything `list` shows.
type ListView struct {
	Accounts  []AccountView
	Threshold int
}

// List refreshes the usage of every inactive account (the active one is fed
// by the status line) and returns the rows to show.
func (s *Service) List(ctx context.Context) (ListView, error) {
	st, err := s.state()
	if err != nil {
		return ListView{}, err
	}
	var wg sync.WaitGroup
	for _, name := range st.Order {
		if name == st.ActiveName() {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.updateUsage(ctx, name)
		}()
	}
	wg.Wait()

	if st, err = s.state(); err != nil {
		return ListView{}, err
	}
	now := s.now()
	view := ListView{Threshold: s.threshold(st)}
	for _, name := range st.Order {
		row := AccountView{
			Name:   name,
			Email:  st.Accounts[name].Email,
			Active: name == st.ActiveName(),
			Dead:   st.IsDead(name, now),
		}
		if u, ok := st.Usage[name]; ok {
			row.Usage = &u
		}
		view.Accounts = append(view.Accounts, row)
	}
	return view, nil
}

// updateUsage reads usage for one inactive account, refreshing its access
// token through Claude Code first when it has expired. Failures keep the last
// known numbers and are only logged.
func (s *Service) updateUsage(ctx context.Context, name string) {
	p, err := s.d.Profiles.Load(name)
	if err != nil {
		return
	}
	if !p.Login.AccessTokenValid(s.now()) {
		if p, err = s.refresh(ctx, name, p); err != nil {
			s.log("refresh %s: %v", name, err)
			return
		}
	}
	u, err := s.d.Usage.Read(ctx, p.Login.AccessToken())
	if err != nil {
		s.log("usage %s: %v", name, err)
		return
	}
	_ = s.update(ctx, func(st *domain.State) error {
		st.SetUsage(name, u)
		return nil
	})
}

// refresh has the official Claude Code binary refresh an inactive account's
// token: the saved login goes into a throwaway config, `claude auth status`
// runs there, and whatever Claude Code wrote back is saved. The live login is
// never touched.
func (s *Service) refresh(ctx context.Context, name string, p domain.Profile) (domain.Profile, error) {
	tmp, cleanup, err := s.d.Homes.Temp()
	if err != nil {
		return p, err
	}
	defer cleanup()
	if err := tmp.WriteLogin(p.Login); err != nil {
		return p, err
	}
	// Claude Code takes the email from the config, not from the token.
	if err := tmp.WriteAccount(p.OAuthAccount); err != nil {
		return p, err
	}
	email, err := tmp.AuthStatus(ctx)
	if err != nil {
		return p, err
	}
	if email != p.Email() {
		return p, errors.New("Claude Code did not load the login")
	}
	fresh, err := tmp.ReadLogin()
	if err != nil {
		return p, err
	}
	if !fresh.AccessTokenValid(s.now()) {
		return p, errors.New("Claude Code did not refresh the token")
	}
	refreshed := domain.NewProfile(fresh, p.OAuthAccount)
	err = s.update(ctx, func(st *domain.State) error {
		return s.saveProfile(st, name, refreshed, email)
	})
	return refreshed, err
}
