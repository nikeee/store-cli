// Package config holds environment variable names and credential loading.
package config

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
