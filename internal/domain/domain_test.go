package domain_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/PuntoyComaTech/claude-switch/internal/domain"
)

var now = time.Unix(1_800_000_000, 0)

func f(v float64) *float64 { return &v }

func TestWithLoginKeepsMachineKeys(t *testing.T) {
	live, _ := domain.ParseCredentials([]byte(`{"claudeAiOauth":{"accessToken":"old"},"mcpOAuth":{"x":1},"organizationUuid":"o1"}`))
	saved, _ := domain.ParseCredentials([]byte(`{"claudeAiOauth":{"accessToken":"new"},"mcpOAuth":{"x":2}}`))
	got := live.WithLogin(saved)
	if got.AccessToken() != "new" {
		t.Errorf("token: %s", got.AccessToken())
	}
	if string(got["mcpOAuth"]) != `{"x":1}` {
		t.Errorf("machine key replaced: %s", got["mcpOAuth"])
	}
	if _, ok := got["organizationUuid"]; ok {
		t.Error("a login key missing from the saved login must not survive from the old one")
	}
}

func TestAccessTokenValid(t *testing.T) {
	cases := map[string]bool{
		`{}`:                                    false,
		`{"claudeAiOauth":{"accessToken":"t"}}`: false,
		`{"claudeAiOauth":{"accessToken":"t","expiresAt":1800000030000}}`: false, // inside the 60 s margin
		`{"claudeAiOauth":{"accessToken":"t","expiresAt":1800000120000}}`: true,
		`{"claudeAiOauth":"garbage"}`:                                     false,
	}
	for in, want := range cases {
		c, err := domain.ParseCredentials([]byte(in))
		if err != nil {
			t.Fatal(err)
		}
		if got := c.AccessTokenValid(now); got != want {
			t.Errorf("%s: got %v want %v", in, got, want)
		}
	}
}

func TestMarshalCompactDoesNotEscapeHTML(t *testing.T) {
	b, _ := domain.MarshalCompact(map[string]string{"a": "x<y&z"})
	if string(b) != `{"a":"x<y&z"}` {
		t.Errorf("got %s", b)
	}
}

func TestValidateName(t *testing.T) {
	for _, ok := range []string{"work", "a.b_c-1", "W0rk"} {
		if err := domain.ValidateName(ok); err != nil {
			t.Errorf("%q rejected: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "two words", "../x", ".hidden", "a/b", "x\"y", "123456789012345678901234567890123"} {
		if domain.ValidateName(bad) == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestProfileRoundTrip(t *testing.T) {
	c, _ := domain.ParseCredentials([]byte(`{"claudeAiOauth":{"accessToken":"t","refreshTokenExpiresAt":5},"mcpOAuth":{}}`))
	p := domain.NewProfile(c, json.RawMessage(`{"emailAddress":"a@x"}`))
	b, _ := p.Bytes()
	q, err := domain.ParseProfile(b)
	if err != nil || q.Email() != "a@x" || q.Login.AccessToken() != "t" {
		t.Fatalf("got %+v, %v", q, err)
	}
	if _, ok := q.Login["mcpOAuth"]; ok {
		t.Error("profiles store only login keys")
	}
	if *q.Login.RefreshTokenExpiry() != 5 {
		t.Error("refresh expiry lost")
	}
	if domain.NewProfile(c, nil).OAuthAccount == nil {
		t.Error("missing account must become JSON null")
	}
}

func TestIsExhaustedAndDead(t *testing.T) {
	st := domain.NewState()
	st.Upsert("a", domain.Account{Email: "a@x", RefreshExpiry: f(1000)})
	future, past := f(float64(now.Unix()+60)), f(float64(now.Unix()-60))

	st.SetUsage("a", domain.Usage{Five: f(96), FiveReset: future})
	if !st.IsExhausted("a", 95, now) {
		t.Error("96% before reset is exhausted")
	}
	st.SetUsage("a", domain.Usage{Five: f(96), FiveReset: past})
	if st.IsExhausted("a", 95, now) {
		t.Error("a window that already reset is not exhausted")
	}
	st.SetUsage("a", domain.Usage{Seven: f(95), SevenReset: future})
	if !st.IsExhausted("a", 95, now) {
		t.Error("weekly at threshold is exhausted")
	}
	if !st.IsDead("a", now) {
		t.Error("expired refresh token is dead")
	}
	if st.IsExhausted("missing", 95, now) || st.IsDead("missing", now) {
		t.Error("unknown accounts are neither")
	}
}

func TestCandidatesRotate(t *testing.T) {
	st := domain.NewState()
	for _, n := range []string{"a", "b", "c"} {
		st.Upsert(n, domain.Account{})
	}
	st.SetActive("b", now)
	got := st.Candidates()
	if len(got) != 2 || got[0] != "c" || got[1] != "a" {
		t.Errorf("got %v", got)
	}
	st.Active = nil
	if got := st.Candidates(); len(got) != 3 || got[0] != "a" {
		t.Errorf("no active: got %v", got)
	}
}

func TestRecordReport(t *testing.T) {
	st := domain.NewState()
	st.Upsert("a", domain.Account{})
	st.SetActive("a", now)
	reset := float64(now.Unix() + 3600)
	line := func(sid string, five float64) domain.StatusLine {
		return domain.StatusLine{SessionID: sid, RateLimits: &domain.RateLimits{
			FiveHour: &domain.Window{UsedPercentage: &five, ResetsAt: &reset},
		}}
	}
	if st.RecordReport(line("s", 50), 95, now) {
		t.Error("50% must not switch")
	}
	if *st.Usage["a"].Five != 50 || st.Usage["a"].Seven == nil || *st.Usage["a"].Seven != 0 {
		t.Errorf("usage not recorded: %+v", st.Usage["a"])
	}
	if !st.RecordReport(line("s", 95), 95, now) {
		t.Error("95% at threshold 95 must switch")
	}
	if st.RecordReport(line("s", 95), 95, now) {
		t.Error("a repeated signature from the same session is ignored")
	}
	if !st.RecordReport(line("other", 95), 95, now) {
		t.Error("another session with the same numbers is new")
	}
	// Sessions older than a day are forgotten.
	st.RecordReport(line("fresh", 1), 95, now.Add(25*time.Hour))
	if _, ok := st.Sessions["s"]; ok {
		t.Error("old session not pruned")
	}
}

func TestStateRenameRemoveThreshold(t *testing.T) {
	st := domain.NewState()
	st.Upsert("a", domain.Account{})
	st.Upsert("b", domain.Account{})
	st.SetActive("a", now)
	st.SetUsage("a", domain.Usage{Five: f(1)})
	if err := st.Rename("a", "b"); err == nil {
		t.Error("renaming onto an existing name must fail")
	}
	if err := st.Rename("a", "main"); err != nil || st.ActiveName() != "main" || st.Usage["main"].Five == nil {
		t.Errorf("rename: %v %+v", err, st)
	}
	if err := st.Remove("main"); err == nil {
		t.Error("removing the active account must fail")
	}
	if err := st.Remove("nope"); err == nil {
		t.Error("removing an unknown account must fail")
	}
	if st.EffectiveThreshold() != domain.DefaultThreshold {
		t.Error("default threshold")
	}
	if st.SetThreshold(101) == nil || st.SetThreshold(0) == nil || st.SetThreshold(80) != nil || st.EffectiveThreshold() != 80 {
		t.Error("threshold validation")
	}
	if st.NameForEmail("x@x") != "" {
		t.Error("unknown email")
	}
}
