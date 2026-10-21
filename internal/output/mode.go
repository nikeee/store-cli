// Package output renders command results as text, JSON or TOON and prints
// errors, progress and warnings.
package output

import (
	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/config"
)

// Mode is the output format.
type Mode string

const (
	ModeText Mode = "text"
	ModeJSON Mode = "json"
	ModeTOON Mode = "toon"
)

// ResolveMode picks the output mode: flag, then STORE_OUTPUT, then toon if
// any agent marker is set (even empty), else text.
func ResolveMode(flag string, env config.Env) (Mode, error) {
	v := flag
	if v == "" {
		v, _ = env(config.EnvOutput)
	}
	if v == "" {
		for _, m := range config.AgentMarkers {
			if _, ok := env(m); ok {
				return ModeTOON, nil
			}
		}
		return ModeText, nil
	}
	switch Mode(v) {
	case ModeText, ModeJSON, ModeTOON:
		return Mode(v), nil
	}
	return "", clierr.Usagef("invalid output %q, use text, json or toon", v)
}
