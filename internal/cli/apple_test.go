package cli_test

import (
	"maps"
	"strings"
	"testing"

	"github.com/nikeee/store-cli/internal/config"
	"github.com/nikeee/store-cli/internal/testutil/ascfake"
)

// appleEnv returns the environment for Apple commands against srv, plus extra.
func appleEnv(srv *ascfake.Server, extra map[string]string) map[string]string {
	env := map[string]string{
		config.EnvAppleIssuerID:       ascfake.IssuerID,
		config.EnvAppleKeyID:          ascfake.KeyID,
		config.EnvApplePrivateKey:     string(ascfake.KeyPEM),
		config.EnvInternalAppleAPIURL: srv.URL,
	}
	maps.Copy(env, extra)
	return env
}

func TestAppleMissingAppExits2(t *testing.T) {
	srv := ascfake.New(t)
	code, _, stderr := runEnv(t, appleEnv(srv, nil), false, "apple", "app", "show")
	if code != 2 || !strings.Contains(stderr, "--app") || !strings.Contains(stderr, config.EnvAppleBundleID) {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	if n := len(srv.Requests()); n != 0 {
		t.Errorf("%d requests sent", n)
	}
}

func TestAppleMissingCredentialsExits1(t *testing.T) {
	srv := ascfake.New(t)
	env := map[string]string{config.EnvInternalAppleAPIURL: srv.URL, config.EnvAppleBundleID: "com.example.app"}
	for _, args := range [][]string{{"auth", "check"}, {"app", "list"}, {"app", "show"}} {
		code, stdout, stderr := runEnv(t, env, false, append([]string{"apple"}, args...)...)
		if code != 1 || stdout != "" || !strings.Contains(stderr, "missing env vars: "+config.EnvAppleIssuerID) {
			t.Errorf("%v: code=%d stdout=%q stderr=%q", args, code, stdout, stderr)
		}
	}
	if n := len(srv.Requests()); n != 0 {
		t.Errorf("%d requests sent", n)
	}
}
