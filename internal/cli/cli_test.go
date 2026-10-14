package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/PuntoyComaTech/claude-switch/internal/app"
	"github.com/PuntoyComaTech/claude-switch/internal/domain"
)

func f(v float64) *float64 { return &v }

func TestRenderList(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	reset := float64(now.Unix() + 2*3600 + 5*60)
	var b bytes.Buffer
	renderList(&b, app.ListView{Threshold: 95, Accounts: []app.AccountView{
		{Name: "work", Email: "w@x", Active: true, Usage: &domain.Usage{Five: f(32.7), FiveReset: &reset, At: float64(now.Unix() - 120)}},
		{Name: "old", Email: "o@x", Dead: true},
	}}, now, false)
	out := b.String()
	for _, want := range []string{"● work", "███░░░░░░░  32%", "resets in 2h 5m", "2m ago", "no data yet", "LOGIN NEEDED: claude-switch login old", "95% of either window"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "\x1b[") {
		t.Error("colors must be off when not requested")
	}

	b.Reset()
	renderList(&b, app.ListView{}, now, false)
	if !strings.Contains(b.String(), "no accounts yet") {
		t.Error("empty list hint")
	}
}

func TestMeterExpiredWindowShowsNoData(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	past := float64(now.Unix() - 1)
	if !strings.Contains(meter(colors(false), f(50), &past, 95, now), "no data yet") {
		t.Error("a window past its reset must show no data")
	}
	future := float64(now.Unix() + 60)
	if !strings.Contains(meter(colors(true), f(96), &future, 95, now), "\x1b[31m") {
		t.Error("over threshold must be red")
	}
}

func TestHumanDurations(t *testing.T) {
	now := time.Unix(0, 0)
	cases := map[time.Duration]string{30 * time.Second: "1m", 90 * time.Minute: "1h 30m", 50 * time.Hour: "2d 2h"}
	for d, want := range cases {
		if got := humanIn(now, now.Add(d)); got != want {
			t.Errorf("humanIn(%v) = %q, want %q", d, got, want)
		}
	}
	if humanAgo(now.Add(10*time.Second), now) != "just now" || humanAgo(now.Add(49*time.Hour), now) != "2d ago" {
		t.Error("humanAgo")
	}
}

func TestStatusText(t *testing.T) {
	var in statusInput
	_ = json.Unmarshal([]byte(`{"model":{"display_name":"Opus"},"rate_limits":{"five_hour":{"used_percentage":40.7},"seven_day":{"used_percentage":12}}}`), &in)
	if got := statusText("work", in); got != "work · Opus · 5h 40% · 7d 12%" {
		t.Errorf("got %q", got)
	}
	if got := statusText("", statusInput{}); got != "?" {
		t.Errorf("empty: got %q", got)
	}
}

func TestSetupStatusLine(t *testing.T) {
	dir := t.TempDir()
	settings := filepath.Join(dir, "settings.json")
	var out bytes.Buffer

	// No settings file: create one with our status line.
	if err := setupStatusLine(&out, settings); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(settings)
	if !strings.Contains(string(b), statusLineCommand) || !strings.Contains(out.String(), "installed") {
		t.Fatalf("not installed: %s / %s", b, out.String())
	}

	// Ours already: idempotent.
	out.Reset()
	_ = setupStatusLine(&out, settings)
	if !strings.Contains(out.String(), "already hooked") {
		t.Errorf("got %q", out.String())
	}

	// A user's own status line is never replaced, and other keys survive.
	script := filepath.Join(dir, "mine.sh")
	_ = os.WriteFile(script, []byte("input=$(cat)\necho hi\n"), 0o600)
	own := `{"model":"opus","statusLine":{"type":"command","command":"bash ` + script + `"}}`
	_ = os.WriteFile(settings, []byte(own), 0o600)
	out.Reset()
	_ = setupStatusLine(&out, settings)
	if b, _ := os.ReadFile(settings); string(b) != own {
		t.Errorf("user settings changed: %s", b)
	}
	if !strings.Contains(out.String(), reportHook) {
		t.Errorf("hook line not printed: %s", out.String())
	}

	// Their script already calls report: recognised as hooked.
	_ = os.WriteFile(script, []byte("input=$(cat)\nprintf '%s' \"$input\" | claude-switch report\n"), 0o600)
	out.Reset()
	_ = setupStatusLine(&out, settings)
	if !strings.Contains(out.String(), "already hooked") {
		t.Errorf("got %q", out.String())
	}
}

func TestSetupBacksUpExistingSettings(t *testing.T) {
	settings := filepath.Join(t.TempDir(), "settings.json")
	_ = os.WriteFile(settings, []byte(`{"model":"opus"}`), 0o600)
	if err := setupStatusLine(&bytes.Buffer{}, settings); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(settings + ".bak.claude-switch"); string(b) != `{"model":"opus"}` {
		t.Errorf("backup: %s", b)
	}
	b, _ := os.ReadFile(settings)
	if !strings.Contains(string(b), `"model": "opus"`) {
		t.Errorf("other keys lost: %s", b)
	}
}

func TestRootHelpTouchesNothing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CLAUDE_SWITCH_DIR", filepath.Join(t.TempDir(), "never"))
	root := newRoot("test")
	root.SetArgs([]string{"--help"})
	root.SetOut(&bytes.Buffer{})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(os.Getenv("CLAUDE_SWITCH_DIR")); !os.IsNotExist(err) {
		t.Error("--help must not create the data folder")
	}
}
