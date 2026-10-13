package app_test

import (
	"context"
	"testing"

	"github.com/PuntoyComaTech/claude-switch/internal/domain"
)

// These scenarios port tests/sandbox.sh from the bash version.

var ctx = context.Background()

func twoAccounts(t *testing.T) *fixture {
	t.Helper()
	f := newFixture(t)
	f.loginAs("a@x", "tokA")
	must(t, f.svc.Add(ctx, "a"))
	f.loginAs("b@x", "tokB")
	must(t, f.svc.Add(ctx, "b"))
	return f
}

func TestAddAndUse(t *testing.T) {
	f := twoAccounts(t)
	eq(t, "add keeps last added active", f.current(), "b")

	f.loginAs("b@x", "tokB2") // Claude Code rotated b's token
	must(t, f.svc.Use(ctx, "a", false))
	eq(t, "use swaps live token", f.liveToken(), "tokA")
	eq(t, "use swaps oauthAccount", f.liveEmail(), "a@x")
	c, _ := f.world.live.ReadLogin()
	eq(t, "use keeps mcpOAuth", string(c["mcpOAuth"]), `{"x":"keep"}`)
	eq(t, "outgoing account saved with rotated token", f.profiles.token("b"), "tokB2")
}

func TestUseRefusesUnsavedManualLogin(t *testing.T) {
	f := twoAccounts(t)
	f.loginAs("c@x", "tokC")
	if err := f.svc.Use(ctx, "a", false); err == nil {
		t.Fatal("expected an error for an unsaved live login")
	}
	eq(t, "live login untouched", f.liveToken(), "tokC")
}

func TestAddRejectsNameAndEmailMismatch(t *testing.T) {
	f := twoAccounts(t)
	f.loginAs("c@x", "tokC")
	if err := f.svc.Add(ctx, "a"); err == nil {
		t.Fatal("'a' is a@x; adding it as c@x must fail")
	}
	if err := f.svc.Add(ctx, "bad name"); err == nil {
		t.Fatal("names with spaces must be rejected")
	}
}

func TestReportSwitchesOverThreshold(t *testing.T) {
	f := twoAccounts(t)
	must(t, f.svc.Use(ctx, "a", false))

	must(t, f.report("s1", 40, 10))
	eq(t, "under threshold stays", f.current(), "a")

	must(t, f.report("s1", 96, 10))
	eq(t, "over threshold switches", f.current(), "b")
	eq(t, "switch put b token live", f.liveToken(), "tokB")

	must(t, f.report("s1", 96, 10))
	eq(t, "stale numbers from same session ignored", f.current(), "b")

	_ = f.report("s2", 97, 20)
	eq(t, "all exhausted stays put", f.current(), "b")
	eq(t, "all exhausted notifies", f.notes.count("every account"), 1)
}

func TestReportWithoutRateLimitsIsIgnored(t *testing.T) {
	f := twoAccounts(t)
	must(t, f.svc.Report(ctx, domain.StatusLine{SessionID: "s"}))
	eq(t, "no usage recorded", len(f.st().Usage), 0)
}

func TestDeadAccounts(t *testing.T) {
	f := twoAccounts(t)
	f.edit(func(st *domain.State) {
		dead := float64(1000)
		a := st.Accounts["a"]
		a.RefreshExpiry = &dead
		st.Accounts["a"] = a
	})
	_ = f.svc.Next(ctx, true)
	eq(t, "auto skips dead account", f.current(), "b")
	eq(t, "auto notifies dead account", f.notes.count("claude-switch login a"), 1)

	f.world.pick = "a@x"
	must(t, f.svc.Use(ctx, "a", false))
	eq(t, "manual use logs in with its email", f.world.loginArgs[0], "a@x")
	eq(t, "re-login saved fresh token", f.profiles.token("a"), "new-a@x")
	eq(t, "re-login then switches", f.liveToken(), "new-a@x")
	eq(t, "dead flag cleared", f.st().IsDead("a", f.now), false)
	eq(t, "temp config removed", f.world.temps, 0)
}

func TestAddWithLogin(t *testing.T) {
	f := twoAccounts(t)
	before := f.liveToken()

	f.world.pick = "c@x"
	must(t, f.svc.AddWithLogin(ctx, "extra"))
	eq(t, "saves the new account", f.profiles.token("extra"), "new-c@x")
	eq(t, "keeps live login", f.liveToken(), before)
	eq(t, "keeps active", f.current(), "b")

	if err := f.svc.AddWithLogin(ctx, "dup"); err == nil {
		t.Fatal("same email under a new name must fail")
	}
	eq(t, "duplicate not saved", f.st().Has("dup"), false)

	f.world.pick = "z@x"
	if err := f.svc.Login(ctx, "extra"); err == nil {
		t.Fatal("login as another account must fail")
	}
	eq(t, "wrong account not saved", f.profiles.token("extra"), "new-c@x")
}

func TestStrayLoginIsRestored(t *testing.T) {
	f := twoAccounts(t)
	before := f.liveToken()
	f.world.pick, f.world.stray = "d@x", true
	// The stray login wrote only the live item, so the email comes from the
	// throwaway config's auth status, which has no account: it must fail
	// without leaving the live login changed.
	_ = f.svc.AddWithLogin(ctx, "stray")
	eq(t, "stray login restores live login", f.liveToken(), before)
	eq(t, "temp config removed", f.world.temps, 0)
}

func TestRenameAndRemove(t *testing.T) {
	f := twoAccounts(t)
	must(t, f.svc.Use(ctx, "a", false))
	must(t, f.svc.Rename(ctx, "b", "work"))
	eq(t, "rename leaves active alone", f.current(), "a")
	eq(t, "rename moves saved login", f.profiles.token("work"), "tokB")
	eq(t, "rename drops old login", f.profiles.token("b"), "")

	must(t, f.svc.Rename(ctx, "a", "main"))
	eq(t, "rename moves active", f.current(), "main")
	eq(t, "rename keeps order", f.st().Order[0]+","+f.st().Order[1], "main,work")

	if err := f.svc.Remove(ctx, "main"); err == nil {
		t.Fatal("removing the active account must fail")
	}
	must(t, f.svc.Remove(ctx, "work"))
	eq(t, "removed from state", f.st().Has("work"), false)
	eq(t, "removed from store", f.profiles.token("work"), "")
}

func TestListReadsInactiveUsage(t *testing.T) {
	f := twoAccounts(t)
	must(t, f.svc.Use(ctx, "a", false))
	// b's saved access token is valid.
	p, _ := f.profiles.Load("b")
	p.Login = mustCreds(t, credsJSON("tokB", 9999999999999, 0))
	must(t, f.profiles.Save("b", p))

	five, reset := 99.0, float64(4070944800)
	f.usage.result = &domain.Usage{Five: &five, FiveReset: &reset, Source: "api"}
	v, err := f.svc.List(ctx)
	must(t, err)
	eq(t, "list shows both", len(v.Accounts), 2)
	eq(t, "list reads usage of inactive account", *f.st().Usage["b"].Five, 99.0)
	eq(t, "active account is not fetched", len(f.usage.calls), 1)
	eq(t, "fetched with the saved token", f.usage.calls[0], "tokB")

	f.usage.calls = nil
	_ = f.svc.Next(ctx, false)
	eq(t, "next skips account known exhausted", f.current(), "a")
	eq(t, "next makes no request", len(f.usage.calls), 0)

	f.usage.result = nil
	_, _ = f.svc.List(ctx)
	eq(t, "failed request keeps last numbers", *f.st().Usage["b"].Five, 99.0)
}

func TestListRefreshesExpiredToken(t *testing.T) {
	f := twoAccounts(t)
	must(t, f.svc.Use(ctx, "a", false))
	// b's saved token is expired (no expiresAt).
	ten := 10.0
	f.usage.result = &domain.Usage{Five: &ten}

	f.world.statusEmail = "z@x"
	_, _ = f.svc.List(ctx)
	eq(t, "refresh refused when Claude Code loads another account", f.profiles.token("b"), "tokB")

	f.world.statusEmail = ""
	_, _ = f.svc.List(ctx)
	eq(t, "nothing saved when Claude Code does not refresh", f.profiles.token("b"), "tokB")
	eq(t, "logged", f.logs.count("did not refresh"), 1)

	f.world.refreshes = true
	_, _ = f.svc.List(ctx)
	eq(t, "claude refreshes expired inactive token", f.profiles.token("b"), "ref-tokB")
	eq(t, "refreshed account gets usage", *f.st().Usage["b"].Five, 10.0)
	eq(t, "refresh leaves live login", f.liveToken(), "tokA")
	eq(t, "temp configs removed", f.world.temps, 0)
}

func TestThreshold(t *testing.T) {
	f := twoAccounts(t)
	must(t, f.svc.Threshold(ctx, "50"))
	eq(t, "threshold is saved", f.st().EffectiveThreshold(), 50)
	if err := f.svc.Threshold(ctx, "0"); err == nil {
		t.Fatal("threshold 0 must be rejected")
	}
	eq(t, "threshold unchanged", f.st().EffectiveThreshold(), 50)
}

func mustCreds(t *testing.T, b []byte) domain.Credentials {
	t.Helper()
	c, err := domain.ParseCredentials(b)
	must(t, err)
	return c
}
