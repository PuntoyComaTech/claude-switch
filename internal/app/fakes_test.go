package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/PuntoyComaTech/claude-switch/internal/app"
	"github.com/PuntoyComaTech/claude-switch/internal/domain"
)

// --- fakes for every port ----------------------------------------------------

type memProfiles struct {
	mu sync.Mutex
	m  map[string][]byte
}

func (p *memProfiles) Load(name string) (domain.Profile, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	b, ok := p.m[name]
	if !ok {
		return domain.Profile{}, app.ErrNotFound
	}
	return domain.ParseProfile(b)
}

func (p *memProfiles) Save(name string, prof domain.Profile) error {
	b, err := prof.Bytes()
	if err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.m[name] = b
	return nil
}

func (p *memProfiles) Delete(name string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.m, name)
	return nil
}

func (p *memProfiles) token(name string) string {
	prof, err := p.Load(name)
	if err != nil {
		return ""
	}
	return prof.Login.AccessToken()
}

type memState struct {
	mu sync.Mutex
	b  []byte
}

func (s *memState) Load() (*domain.State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.b == nil {
		return domain.NewState(), nil
	}
	st := &domain.State{}
	if err := json.Unmarshal(s.b, st); err != nil {
		return nil, err
	}
	st.Normalize()
	return st, nil
}

func (s *memState) Save(st *domain.State) error {
	b, err := json.Marshal(st)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.b = b
	return err
}

type memLock struct{ mu sync.Mutex }

func (l *memLock) Lock(context.Context) (func(), error) {
	l.mu.Lock()
	return l.mu.Unlock, nil
}

// fakeHome is one Claude Code config. login/account are raw JSON.
type fakeHome struct {
	mu      sync.Mutex
	login   []byte
	account json.RawMessage
	world   *world
}

func (h *fakeHome) ReadLogin() (domain.Credentials, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.login == nil {
		return nil, app.ErrNotFound
	}
	return domain.ParseCredentials(h.login)
}

func (h *fakeHome) WriteLogin(c domain.Credentials) error {
	b, err := c.Bytes()
	h.mu.Lock()
	defer h.mu.Unlock()
	h.login = b
	return err
}

func (h *fakeHome) ReadAccount() (json.RawMessage, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.account == nil {
		return json.RawMessage("null"), nil
	}
	return h.account, nil
}

func (h *fakeHome) WriteAccount(a json.RawMessage) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if string(a) == "null" {
		a = nil
	}
	h.account = a
	return nil
}

// AuthStatus behaves like Claude Code: the email comes from the config's
// oauthAccount, and an expired token is refreshed when refreshes is set.
func (h *fakeHome) AuthStatus(context.Context) (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.login == nil {
		return "", nil
	}
	email := domain.AccountEmail(h.account)
	if email == "" {
		return "", nil
	}
	if h.world.statusEmail != "" {
		email = h.world.statusEmail
	}
	if h.world.refreshes {
		c, _ := domain.ParseCredentials(h.login)
		var o map[string]any
		_ = json.Unmarshal(c["claudeAiOauth"], &o)
		o["accessToken"] = "ref-" + fmt.Sprint(o["accessToken"])
		o["expiresAt"] = float64(9999999999999)
		c["claudeAiOauth"], _ = json.Marshal(o)
		h.login, _ = c.Bytes()
	}
	return email, nil
}

// AuthLogin simulates the browser login as world.pick.
func (h *fakeHome) AuthLogin(_ context.Context, email string) error {
	h.world.loginArgs = append(h.world.loginArgs, email)
	pick := h.world.pick
	if pick == "" {
		return errors.New("browser login cancelled")
	}
	target := h
	if h.world.stray { // a Claude Code version that ignores the config dir
		target = h.world.live
	}
	target.mu.Lock()
	target.login = credsJSON("new-"+pick, 9999999999999, 0)
	target.mu.Unlock()
	if !h.world.stray {
		h.mu.Lock()
		h.account = acctJSON(pick)
		h.mu.Unlock()
	}
	return nil
}

type world struct {
	live        *fakeHome
	temps       int
	pick        string
	stray       bool
	statusEmail string
	refreshes   bool
	loginArgs   []string
}

func (w *world) Live() app.Home { return w.live }

func (w *world) Temp() (app.Home, func(), error) {
	w.temps++
	h := &fakeHome{world: w}
	return h, func() { w.temps-- }, nil
}

type fakeUsage struct {
	mu     sync.Mutex
	calls  []string
	result *domain.Usage
}

func (u *fakeUsage) Read(_ context.Context, token string) (domain.Usage, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.calls = append(u.calls, token)
	if u.result == nil {
		return domain.Usage{}, errors.New("HTTP 500")
	}
	return *u.result, nil
}

type recorder struct {
	mu    sync.Mutex
	lines []string
}

func (r *recorder) Notify(m string) { r.add(m) }
func (r *recorder) Log(m string)    { r.add(m) }
func (r *recorder) add(m string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lines = append(r.lines, m)
}

func (r *recorder) count(sub string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, l := range r.lines {
		if strings.Contains(l, sub) {
			n++
		}
	}
	return n
}

// --- fixture -----------------------------------------------------------------

type fixture struct {
	t        *testing.T
	svc      *app.Service
	profiles *memProfiles
	state    *memState
	world    *world
	usage    *fakeUsage
	notes    *recorder
	logs     *recorder
	now      time.Time
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{
		t:        t,
		profiles: &memProfiles{m: map[string][]byte{}},
		state:    &memState{},
		usage:    &fakeUsage{},
		notes:    &recorder{},
		logs:     &recorder{},
		now:      time.Unix(1_800_000_000, 0),
	}
	f.world = &world{}
	f.world.live = &fakeHome{world: f.world}
	f.svc = app.New(app.Deps{
		Profiles: f.profiles, State: f.state, Lock: &memLock{}, Homes: f.world,
		Usage: f.usage, Notifier: f.notes, Logger: f.logs, Now: func() time.Time { return f.now },
	})
	return f
}

// loginAs makes Claude Code's live login email/token, with a machine key that
// must survive swaps.
func (f *fixture) loginAs(email, token string) {
	c, _ := domain.ParseCredentials(credsJSON(token, 0, 9999999999999))
	c["mcpOAuth"] = json.RawMessage(`{"x":"keep"}`)
	f.world.live.login, _ = c.Bytes()
	f.world.live.account = acctJSON(email)
}

func (f *fixture) liveToken() string {
	c, _ := f.world.live.ReadLogin()
	return c.AccessToken()
}

func (f *fixture) liveEmail() string { return domain.AccountEmail(f.world.live.account) }

func (f *fixture) current() string {
	st, _ := f.state.Load()
	return st.ActiveName()
}

func (f *fixture) st() *domain.State {
	st, _ := f.state.Load()
	return st
}

func (f *fixture) edit(fn func(*domain.State)) {
	st := f.st()
	fn(st)
	_ = f.state.Save(st)
}

func (f *fixture) report(session string, five, seven float64) error {
	reset := float64(f.now.Unix() + 3600)
	week := reset + 86400
	return f.svc.Report(context.Background(), domain.StatusLine{
		SessionID: session,
		RateLimits: &domain.RateLimits{
			FiveHour: &domain.Window{UsedPercentage: &five, ResetsAt: &reset},
			SevenDay: &domain.Window{UsedPercentage: &seven, ResetsAt: &week},
		},
	})
}

func credsJSON(token string, expiresAt, rtExp float64) []byte {
	o := map[string]any{"accessToken": token}
	if expiresAt > 0 {
		o["expiresAt"] = expiresAt
	}
	if rtExp > 0 {
		o["refreshTokenExpiresAt"] = rtExp
	} else {
		o["refreshTokenExpiresAt"] = float64(9999999999999)
	}
	b, _ := json.Marshal(map[string]any{"claudeAiOauth": o})
	return b
}

func acctJSON(email string) json.RawMessage {
	b, _ := json.Marshal(map[string]string{"emailAddress": email})
	return b
}

func eq[T comparable](t *testing.T, name string, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %v, want %v", name, got, want)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
