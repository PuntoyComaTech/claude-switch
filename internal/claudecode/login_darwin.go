package claudecode

import (
	"context"
	"errors"

	"github.com/PuntoyComaTech/claude-switch/internal/app"
	"github.com/PuntoyComaTech/claude-switch/internal/keychain"
	"github.com/PuntoyComaTech/claude-switch/internal/sysexec"
)

// On macOS Claude Code keeps the login in the Keychain.
type keychainLogin struct {
	kc      keychain.Keychain
	service string
	account string
}

func newLoginStore(l Layout, run sysexec.Runner) loginStore {
	return keychainLogin{kc: keychain.Keychain{Run: run}, service: l.KeychainService, account: l.KeychainAccount}
}

func (k keychainLogin) read(ctx context.Context) ([]byte, error) {
	b, err := k.kc.Get(ctx, k.service, k.account)
	if errors.Is(err, keychain.ErrNotFound) {
		return nil, app.ErrNotFound
	}
	return b, err
}

func (k keychainLogin) write(ctx context.Context, b []byte) error {
	return k.kc.Set(ctx, k.service, k.account, b)
}

func (k keychainLogin) cleanup(ctx context.Context) {
	_ = k.kc.Delete(ctx, k.service, k.account)
}

func checkSupported(Env, Layout) error { return nil }
