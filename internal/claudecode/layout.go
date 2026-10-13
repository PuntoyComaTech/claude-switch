// Package claudecode adapts Claude Code's on-disk login and config to the
// app.Home port. It mirrors the storage rules of Claude Code 2.1.x:
//
//   - the login lives in the macOS Keychain, or in <secure dir>/.credentials.json
//     (mode 0600) on Linux and Windows;
//   - the account (oauthAccount) lives in ~/.claude.json, or in
//     $CLAUDE_CONFIG_DIR/.claude.json, and a legacy <config dir>/.config.json
//     wins when it exists;
//   - running sessions notice a new login when the mtime of
//     <secure dir>/.credentials.json changes.
package claudecode

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"

	"golang.org/x/text/unicode/norm"
)

const (
	keychainBase  = "Claude Code-credentials"
	credsFileName = ".credentials.json"
)

// Env reads environment variables; os.LookupEnv in production.
type Env func(key string) (string, bool)

// Layout is where one Claude Code configuration keeps its files and secrets.
type Layout struct {
	// ConfigDir is Claude Code's config directory (~/.claude or $CLAUDE_CONFIG_DIR).
	ConfigDir string
	// SecureDir holds .credentials.json.
	SecureDir string
	// ConfigFile holds oauthAccount.
	ConfigFile string
	// KeychainService and KeychainAccount name the macOS login item.
	KeychainService string
	KeychainAccount string
	// RunEnv is applied when running `claude` for this configuration.
	RunEnv []string
}

// CredentialsFile is the login file, or the file whose mtime signals a new
// login on macOS.
func (l Layout) CredentialsFile() string { return filepath.Join(l.SecureDir, credsFileName) }

// LiveLayout resolves the configuration Claude Code uses in this environment.
func LiveLayout(env Env, home, user string) Layout {
	cfgDir, hasCfg := env("CLAUDE_CONFIG_DIR")
	hasCfg = hasCfg && cfgDir != ""
	if !hasCfg {
		cfgDir = filepath.Join(home, ".claude")
	}
	secure, hasSecure := env("CLAUDE_SECURESTORAGE_CONFIG_DIR")
	keyDir, suffixed := cfgDir, hasCfg
	switch {
	case hasSecure && secure != "":
		keyDir, suffixed = secure, true
	case hasSecure:
		// Set but empty: Claude Code falls back to ~/.claude with no suffix.
		secure, suffixed = filepath.Join(home, ".claude"), false
	default:
		secure = cfgDir
	}
	cfgFile := filepath.Join(home, ".claude.json")
	if hasCfg {
		cfgFile = filepath.Join(cfgDir, ".claude.json")
	}
	return Layout{
		ConfigDir:       cfgDir,
		SecureDir:       secure,
		ConfigFile:      preferLegacy(cfgDir, cfgFile),
		KeychainService: serviceName(keyDir, suffixed),
		KeychainAccount: keychainAccount(user),
	}
}

// TempLayout is a throwaway configuration in dir, isolated from the live one.
func TempLayout(dir, user string) Layout {
	return Layout{
		ConfigDir:       dir,
		SecureDir:       dir,
		ConfigFile:      filepath.Join(dir, ".claude.json"),
		KeychainService: serviceName(dir, true),
		KeychainAccount: keychainAccount(user),
		RunEnv:          []string{"CLAUDE_CONFIG_DIR=" + dir, "CLAUDE_SECURESTORAGE_CONFIG_DIR="},
	}
}

func preferLegacy(cfgDir, cfgFile string) string {
	legacy := filepath.Join(cfgDir, ".config.json")
	if _, err := os.Stat(legacy); err == nil {
		return legacy
	}
	return cfgFile
}

// serviceName is Claude Code's Keychain service: the base name, plus the first
// 8 hex chars of sha256(dir) when a custom config dir is in use.
func serviceName(dir string, suffixed bool) string {
	if !suffixed {
		return keychainBase
	}
	sum := sha256.Sum256([]byte(norm.NFC.String(dir)))
	return keychainBase + "-" + hex.EncodeToString(sum[:])[:8]
}

var safeUser = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

func keychainAccount(user string) string {
	if safeUser.MatchString(user) {
		return user
	}
	return "claude-code-user"
}
