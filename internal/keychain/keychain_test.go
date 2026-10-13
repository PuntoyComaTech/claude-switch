package keychain_test

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/PuntoyComaTech/claude-switch/internal/keychain"
	"github.com/PuntoyComaTech/claude-switch/internal/sysexec"
)

// fakeSecurity mimics /usr/bin/security for generic passwords and records
// every argv and stdin it receives.
type fakeSecurity struct {
	items map[string][]byte
	argv  [][]string
	stdin [][]byte
}

var addLine = regexp.MustCompile(`^add-generic-password -U -a "([^"]*)" -s "([^"]*)" -X "([0-9a-f]*)"\n$`)

func (f *fakeSecurity) Run(_ context.Context, c sysexec.Cmd) ([]byte, error) {
	f.argv = append(f.argv, c.Args)
	f.stdin = append(f.stdin, c.Stdin)
	arg := func(flag string) string {
		for i, a := range c.Args {
			if a == flag && i+1 < len(c.Args) {
				return c.Args[i+1]
			}
		}
		return ""
	}
	switch c.Args[0] {
	case "-i":
		m := addLine.FindStringSubmatch(string(c.Stdin))
		if m == nil {
			return nil, &sysexec.ExitError{Code: 1}
		}
		b, _ := hex.DecodeString(m[3])
		f.items[m[2]+"/"+m[1]] = b
	case "add-generic-password":
		b, _ := hex.DecodeString(arg("-X"))
		f.items[arg("-s")+"/"+arg("-a")] = b
	case "find-generic-password":
		b, ok := f.items[arg("-s")+"/"+arg("-a")]
		if !ok {
			return nil, &sysexec.ExitError{Code: 44}
		}
		return append(b, '\n'), nil
	case "delete-generic-password":
		key := arg("-s") + "/" + arg("-a")
		if _, ok := f.items[key]; !ok {
			return nil, &sysexec.ExitError{Code: 44}
		}
		delete(f.items, key)
	}
	return nil, nil
}

func newKC() (keychain.Keychain, *fakeSecurity) {
	f := &fakeSecurity{items: map[string][]byte{}}
	return keychain.Keychain{Run: f}, f
}

func TestSetUsesStdinSoSecretsStayOffArgv(t *testing.T) {
	kc, f := newKC()
	secret := []byte(`{"claudeAiOauth":{"accessToken":"sk-secret"}}`)
	if err := kc.Set(context.Background(), "claude-switch", "work", secret); err != nil {
		t.Fatal(err)
	}
	for _, a := range f.argv {
		joined := strings.Join(a, " ")
		if strings.Contains(joined, "sk-secret") || strings.Contains(joined, hex.EncodeToString(secret)) {
			t.Fatalf("secret on argv: %q", joined)
		}
	}
	got, err := kc.Get(context.Background(), "claude-switch", "work")
	if err != nil || !bytes.Equal(got, secret) {
		t.Fatalf("round trip: %q, %v", got, err)
	}
}

func TestSetFallsBackToArgvForLargePayloads(t *testing.T) {
	kc, f := newKC()
	big := bytes.Repeat([]byte("x"), 3000) // 6000 hex chars: over security -i's line limit
	if err := kc.Set(context.Background(), "Claude Code-credentials", "me", big); err != nil {
		t.Fatal(err)
	}
	if f.argv[0][0] != "add-generic-password" {
		t.Fatalf("expected argv fallback, got %v", f.argv[0][:1])
	}
}

func TestGetMissingAndDelete(t *testing.T) {
	kc, _ := newKC()
	if _, err := kc.Get(context.Background(), "s", "a"); !errors.Is(err, keychain.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	if err := kc.Delete(context.Background(), "s", "a"); err != nil {
		t.Fatalf("deleting a missing item is not an error: %v", err)
	}
}

func TestSetRejectsNamesThatBreakTheStdinLine(t *testing.T) {
	kc, _ := newKC()
	if err := kc.Set(context.Background(), "svc", `a"b`, []byte("x")); err == nil {
		t.Fatal("a quote in the account must be rejected")
	}
}
