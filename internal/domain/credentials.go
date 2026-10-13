// Package domain holds the rules of claude-switch: saved accounts, usage
// windows, and how a saved login replaces the live one. It does no I/O.
package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

// loginKeys are the keys of Claude Code's credential JSON that belong to a
// login. Every other key (mcpOAuth, ...) belongs to the machine and is kept
// when an account is swapped in.
var loginKeys = []string{"claudeAiOauth", "organizationUuid", "trustedDeviceToken", "enterpriseGateway"}

// tokenMargin is how long before expiry an access token is treated as expired.
const tokenMargin = 60 * time.Second

// Credentials is Claude Code's credential JSON, kept as raw values so unknown
// keys survive a round trip byte for byte.
type Credentials map[string]json.RawMessage

// ParseCredentials decodes Claude Code's credential JSON.
func ParseCredentials(b []byte) (Credentials, error) {
	var c Credentials
	if err := json.Unmarshal(bytes.TrimSpace(b), &c); err != nil {
		return nil, fmt.Errorf("parse credentials: %w", err)
	}
	if c == nil {
		c = Credentials{}
	}
	return c, nil
}

// Bytes encodes the credentials as compact JSON, the way Claude Code writes them.
func (c Credentials) Bytes() ([]byte, error) {
	return MarshalCompact(c)
}

// LoginOnly returns the subset of keys that identify a login.
func (c Credentials) LoginOnly() Credentials {
	out := Credentials{}
	for _, k := range loginKeys {
		if v, ok := c[k]; ok {
			out[k] = v
		}
	}
	return out
}

// WithLogin keeps this machine's non-login keys and takes every login key
// from login.
func (c Credentials) WithLogin(login Credentials) Credentials {
	out := Credentials{}
	for k, v := range c {
		if !isLoginKey(k) {
			out[k] = v
		}
	}
	for k, v := range login.LoginOnly() {
		out[k] = v
	}
	return out
}

type oauth struct {
	AccessToken           string   `json:"accessToken"`
	ExpiresAt             *float64 `json:"expiresAt"`
	RefreshTokenExpiresAt *float64 `json:"refreshTokenExpiresAt"`
}

func (c Credentials) oauth() oauth {
	var o oauth
	if raw, ok := c["claudeAiOauth"]; ok {
		_ = json.Unmarshal(raw, &o) // a malformed block reads as "no token"
	}
	return o
}

// AccessToken returns the OAuth access token, or "".
func (c Credentials) AccessToken() string { return c.oauth().AccessToken }

// AccessTokenValid reports whether the access token outlives now by a margin.
func (c Credentials) AccessTokenValid(now time.Time) bool {
	o := c.oauth()
	if o.AccessToken == "" || o.ExpiresAt == nil {
		return false
	}
	return time.UnixMilli(int64(*o.ExpiresAt)).After(now.Add(tokenMargin))
}

// RefreshTokenExpiry returns the refresh token expiry in Unix milliseconds,
// or nil when Claude Code did not record one.
func (c Credentials) RefreshTokenExpiry() *float64 { return c.oauth().RefreshTokenExpiresAt }

func isLoginKey(k string) bool {
	for _, l := range loginKeys {
		if k == l {
			return true
		}
	}
	return false
}

// MarshalCompact encodes v as compact JSON without HTML escaping, so strings
// such as "a<b" are written exactly as Claude Code would write them.
func MarshalCompact(v any) ([]byte, error) { return marshal(v, "") }

// MarshalIndent encodes v with two-space indentation, like Claude Code's
// config files.
func MarshalIndent(v any) ([]byte, error) { return marshal(v, "  ") }

func marshal(v any, indent string) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", indent)
	if err := enc.Encode(v); err != nil {
		return nil, fmt.Errorf("encode json: %w", err)
	}
	if indent == "" {
		return bytes.TrimRight(buf.Bytes(), "\n"), nil
	}
	return buf.Bytes(), nil
}
