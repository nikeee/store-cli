// Package config holds environment variable names and credential loading.
package config

import "strings"

// Env looks up an environment variable; os.LookupEnv in production.
type Env func(name string) (string, bool)

const (
	EnvAppleIssuerID        = "APPLE_APPSTORE_ISSUER_ID"
	EnvAppleKeyID           = "APPLE_APPSTORE_API_KEY_ID"
	EnvApplePrivateKey      = "APPLE_APPSTORE_API_PRIVATE_KEY"
	EnvAppleBundleID        = "APPLE_APPSTORE_BUNDLE_ID"
	EnvGoogleServiceAccount = "GOOGLE_PLAY_SERVICE_ACCOUNT_JSON"
	EnvGooglePackageName    = "GOOGLE_PLAY_PACKAGE_NAME"
	EnvOutput               = "STORE_OUTPUT"
	EnvNoColor              = "NO_COLOR"
	EnvGitHubActions        = "GITHUB_ACTIONS"   // "true" on GitHub Actions runners
	EnvGitHubWorkspace      = "GITHUB_WORKSPACE" // checkout directory on GitHub Actions runners

	// Test-only API base URL overrides. Never documented for users.
	EnvInternalAppleAPIURL  = "STORE_INTERNAL_APPLE_API_URL"
	EnvInternalGoogleAPIURL = "STORE_INTERNAL_GOOGLE_API_URL"
)

// Var documents an environment variable for users: UsedBy is the command
// group that reads it ("store apple", "store google") or "" for all, Content
// what it holds, Credential whether it is part of UsedBy's credentials.
type Var struct {
	Name, UsedBy, Content string
	Credential            bool
}

// Vars are the documented environment variables (not the test-only ones).
var Vars = []Var{
	{EnvAppleIssuerID, "store apple", "Issuer ID from App Store Connect, Users and Access, Integrations", true},
	{EnvAppleKeyID, "store apple", "Key ID of the team API key", true},
	{EnvApplePrivateKey, "store apple", "The `.p8` key: PEM content, or a path to the file", true},
	{EnvAppleBundleID, "store apple", "Default for `--app`", false},
	{EnvGoogleServiceAccount, "store google", "Service account key: JSON content, or a path to the file; the account needs access to the app in Play Console", true},
	{EnvGooglePackageName, "store google", "Default for `--app`", false},
	{EnvOutput, "", "Default for `--output`", false},
	{EnvNoColor, "", "Disables colors in text output", false},
	{EnvGitHubActions, "", "`true` (set by GitHub Actions): errors and warnings also become annotations", false},
	{EnvGitHubWorkspace, "", "Set by GitHub Actions: annotations name files relative to it", false},
}

// CredentialHelp lists the credential variables of the command group usedBy,
// one "- NAME: content" line each, for its help text.
func CredentialHelp(usedBy string) string {
	var lines []string
	for _, v := range Vars {
		if v.UsedBy == usedBy && v.Credential {
			lines = append(lines, "- "+v.Name+": "+strings.ReplaceAll(v.Content, "`", ""))
		}
	}
	return strings.Join(lines, "\n")
}

// AgentMarkers are environment variables set by AI coding agents. When any
// is set, the CLI treats the session as non-interactive agent use.
var AgentMarkers = []string{
	"AI_AGENT",        // Vercel detect-agent, https://github.com/vercel/vercel/tree/main/packages/detect-agent
	"CLAUDECODE",      // set by Claude Code; also listed by https://docs.rs/crate/is-ai-agent/0.5.0
	"CURSOR_AGENT",    // https://docs.rs/crate/is-ai-agent/0.5.0 and https://packagist.org/packages/laravel/agent-detector
	"GEMINI_CLI",      // https://docs.rs/crate/is-ai-agent/0.5.0 and https://packagist.org/packages/laravel/agent-detector
	"CODEX_SANDBOX",   // https://docs.rs/crate/is-ai-agent/0.5.0 and https://packagist.org/packages/laravel/agent-detector
	"CODEX_THREAD_ID", // https://docs.rs/crate/is-ai-agent/0.5.0 and https://packagist.org/packages/laravel/agent-detector
}
