package domain

import (
	"encoding/json"
	"fmt"
	"regexp"
)

// Profile is a saved account: its login plus the oauthAccount block Claude Code
// keeps in .claude.json. The JSON shape matches the bash version's Keychain items.
type Profile struct {
	Login        Credentials     `json:"login"`
	OAuthAccount json.RawMessage `json:"oauthAccount"`
}

// NewProfile captures the login part of live credentials and its account block.
func NewProfile(live Credentials, account json.RawMessage) Profile {
	if len(account) == 0 {
		account = json.RawMessage("null")
	}
	return Profile{Login: live.LoginOnly(), OAuthAccount: account}
}

// ParseProfile decodes a saved profile.
func ParseProfile(b []byte) (Profile, error) {
	var p Profile
	if err := json.Unmarshal(b, &p); err != nil {
		return Profile{}, fmt.Errorf("parse profile: %w", err)
	}
	if p.Login == nil {
		p.Login = Credentials{}
	}
	return p, nil
}

// Bytes encodes the profile as compact JSON.
func (p Profile) Bytes() ([]byte, error) { return MarshalCompact(p) }

// Email returns the account email recorded in the profile, or "".
func (p Profile) Email() string { return AccountEmail(p.OAuthAccount) }

// AccountEmail reads emailAddress from an oauthAccount block.
func AccountEmail(account json.RawMessage) string {
	var a struct {
		EmailAddress string `json:"emailAddress"`
	}
	if len(account) == 0 || json.Unmarshal(account, &a) != nil {
		return ""
	}
	return a.EmailAddress
}

var accountName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,31}$`)

// ValidateName checks that an account name is one short word, safe as a file
// name and as a Keychain account on every platform.
func ValidateName(name string) error {
	if !accountName.MatchString(name) {
		return fmt.Errorf("invalid account name %q: use one word of letters, digits, '.', '_' or '-' (max 32)", name)
	}
	return nil
}
