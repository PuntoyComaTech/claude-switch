package keychain_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/PuntoyComaTech/claude-switch/internal/keychain"
	"github.com/PuntoyComaTech/claude-switch/internal/sysexec"
)

// TestRealKeychain runs against the login Keychain with a throwaway service.
// It never touches Claude Code's item. Opt in: CLAUDE_SWITCH_KEYCHAIN_TEST=1.
func TestRealKeychain(t *testing.T) {
	if os.Getenv("CLAUDE_SWITCH_KEYCHAIN_TEST") != "1" {
		t.Skip("set CLAUDE_SWITCH_KEYCHAIN_TEST=1 to use the real Keychain")
	}
	kc := keychain.Keychain{Run: sysexec.OS{}}
	ctx := context.Background()
	service := fmt.Sprintf("claude-switch-test-%d", time.Now().UnixNano())
	t.Cleanup(func() { _ = kc.Delete(ctx, service, "probe") })

	for _, size := range []int{1300, 18500} { // a saved profile; Claude Code's live item
		secret := bytes.Repeat([]byte("x"), size)
		copy(secret, `{"claudeAiOauth":{"accessToken":"fake"},"pad":"`)
		if err := kc.Set(ctx, service, "probe", secret); err != nil {
			t.Fatalf("set %d bytes: %v", size, err)
		}
		got, err := kc.Get(ctx, service, "probe")
		if err != nil || !bytes.Equal(got, secret) {
			t.Fatalf("round trip %d bytes failed: %v", size, err)
		}
	}
}
