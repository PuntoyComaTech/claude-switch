package store

import (
	"context"
	"errors"

	"github.com/PuntoyComaTech/claude-switch/internal/app"
	"github.com/PuntoyComaTech/claude-switch/internal/domain"
	"github.com/PuntoyComaTech/claude-switch/internal/keychain"
)

// KeychainService is the Keychain service that holds saved profiles. The bash
// version used the same name, so existing accounts keep working.
const KeychainService = "claude-switch"

// KeychainProfiles keeps each profile as a macOS Keychain item.
type KeychainProfiles struct{ KC keychain.Keychain }

var _ app.ProfileStore = KeychainProfiles{}

// Load implements app.ProfileStore.
func (k KeychainProfiles) Load(name string) (domain.Profile, error) {
	b, err := k.KC.Get(context.Background(), KeychainService, name)
	if errors.Is(err, keychain.ErrNotFound) {
		return domain.Profile{}, app.ErrNotFound
	}
	if err != nil {
		return domain.Profile{}, err
	}
	return domain.ParseProfile(b)
}

// Save implements app.ProfileStore.
func (k KeychainProfiles) Save(name string, p domain.Profile) error {
	if err := domain.ValidateName(name); err != nil {
		return err
	}
	b, err := p.Bytes()
	if err != nil {
		return err
	}
	return k.KC.Set(context.Background(), KeychainService, name, b)
}

// Delete implements app.ProfileStore.
func (k KeychainProfiles) Delete(name string) error {
	return k.KC.Delete(context.Background(), KeychainService, name)
}
