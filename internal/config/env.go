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

	// Test-only API base URL overrides. Never documented for users.
	EnvInternalAppleAPIURL  = "STORE_INTERNAL_APPLE_API_URL"
	EnvInternalGoogleAPIURL = "STORE_INTERNAL_GOOGLE_API_URL"
)

// AgentMarkers are environment variables set by AI coding agents. When any
// is set, the CLI treats the session as non-interactive agent use.
var AgentMarkers = []string{
	"AI_AGENT",      // generic convention, https://github.com/vercel/vercel/tree/main/packages/detect-agent
	"CLAUDECODE",    // Claude Code, https://code.claude.com/docs/en/settings (environment variables)
	"GEMINI_CLI",    // Gemini CLI, https://github.com/google-gemini/gemini-cli (set in spawned shells)
	"CURSOR_AGENT",  // Cursor agent terminal, https://cursor.com/docs/agent/terminal
	"CODEX_SANDBOX", // OpenAI Codex CLI, https://github.com/openai/codex (sandboxed command env)
}
