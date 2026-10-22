package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/nikeee/store-cli/internal/cli"
	"github.com/nikeee/store-cli/internal/testutil/golden"
)

func run(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errb bytes.Buffer
	d := cli.DefaultDeps()
	d.Stdout, d.Stderr = &out, &errb
	code = cli.Run(context.Background(), args, d)
	return code, out.String(), errb.String()
}

func TestRunUnknownCommandExits2(t *testing.T) {
	code, _, stderr := run(t, "nope")
	if code != 2 || !strings.Contains(stderr, "unknown command") {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	code, _, stderr = run(t, "apple", "nope")
	if code != 2 || !strings.Contains(stderr, "unknown command") {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
}

func TestRunUnknownFlagExits2(t *testing.T) {
	code, _, stderr := run(t, "version", "--nope")
	if code != 2 || !strings.Contains(stderr, "unknown flag") {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
}

// Errors raised before cobra parsed the flags still honor --output.
func TestRunEarlyUsageErrorHonorsOutputFlag(t *testing.T) {
	for _, args := range [][]string{{"version", "--nope", "-o", "json"}, {"-o", "json", "nope"}, {"apple", "nope", "--output=json"}} {
		code, _, stderr := runEnv(t, nil, false, args...)
		var e struct {
			Error string `json:"error"`
		}
		if code != 2 || json.Unmarshal([]byte(stderr), &e) != nil || e.Error == "" {
			t.Errorf("%v: code=%d stderr=%q", args, code, stderr)
		}
	}
}

func TestVersionCommand(t *testing.T) {
	code, stdout, _ := run(t, "version")
	if code != 0 || !strings.Contains(stdout, "dev") {
		t.Fatalf("code=%d stdout=%q", code, stdout)
	}
}

func TestHelpListsGroups(t *testing.T) {
	_, stdout, _ := run(t, "--help")
	for _, w := range []string{"apple", "google", "version", "completion"} {
		if !strings.Contains(stdout, w) {
			t.Errorf("help lacks %q: %s", w, stdout)
		}
	}
}

func TestRunInvalidPlatformExits2(t *testing.T) {
	code, _, stderr := run(t, "apple", "--platform", "android", "version")
	if code != 2 || !strings.Contains(stderr, "--platform") {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
}

func TestRunArgValidationExits2(t *testing.T) {
	for _, args := range [][]string{{"version", "extra"}, {"completion", "nope"}} {
		code, _, stderr := run(t, args...)
		if code != 2 {
			t.Errorf("%v: code=%d stderr=%q", args, code, stderr)
		}
	}
}

func runEnv(t *testing.T, env map[string]string, tty bool, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errb bytes.Buffer
	d := cli.DefaultDeps()
	d.Stdout, d.Stderr, d.IsTTY = &out, &errb, tty
	d.Env = func(k string) (string, bool) { v, ok := env[k]; return v, ok }
	code = cli.Run(context.Background(), args, d)
	return code, out.String(), errb.String()
}

func TestRunUsesAgentMode(t *testing.T) {
	code, stdout, stderr := runEnv(t, map[string]string{"CLAUDECODE": "1"}, false, "version")
	if code != 0 || stderr != "" {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	golden.Assert(t, "version.toon", []byte(stdout))
}

func TestVersionJSON(t *testing.T) {
	code, stdout, _ := runEnv(t, nil, false, "version", "-o", "json")
	var got map[string]string
	if err := json.Unmarshal([]byte(stdout), &got); err != nil || code != 0 || got["version"] != "dev" || got["commit"] != "none" || got["date"] != "unknown" {
		t.Fatalf("code=%d got=%v err=%v", code, got, err)
	}
}

func TestRunInvalidOutputExits2(t *testing.T) {
	code, _, stderr := runEnv(t, nil, false, "version", "-o", "xml")
	if code != 2 || stderr != "error: invalid output \"xml\", use text, json or toon\n" {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	// env says json, the invalid flag error is printed as json
	code, _, stderr = runEnv(t, map[string]string{"STORE_OUTPUT": "json"}, false, "version", "--nope")
	if code != 2 || !strings.HasPrefix(stderr, "{") {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
}

func TestRunErrorFormatFollowsMode(t *testing.T) {
	code, _, stderr := runEnv(t, nil, false, "apple", "--platform", "android", "-o", "json", "version")
	if code != 2 || !strings.HasPrefix(stderr, "{\n  \"error\": \"invalid --platform") {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
}

// Both the root hook (output mode) and the apple group hook (platform) must run.
func TestRunRootAndGroupHooksBothRun(t *testing.T) {
	code, _, stderr := runEnv(t, nil, false, "apple", "--platform", "android", "version")
	if code != 2 || !strings.Contains(stderr, "--platform") {
		t.Fatalf("group hook skipped: code=%d stderr=%q", code, stderr)
	}
	code, _, stderr = runEnv(t, nil, false, "apple", "--platform", "ios", "-o", "xml", "version")
	if code != 2 || !strings.Contains(stderr, "invalid output") {
		t.Fatalf("root hook skipped: code=%d stderr=%q", code, stderr)
	}
}

func TestRunColor(t *testing.T) {
	_, stdout, _ := runEnv(t, nil, true, "version")
	if !strings.Contains(stdout, "\x1b[") {
		t.Fatalf("expected color on a TTY: %q", stdout)
	}
	for name, env := range map[string]map[string]string{"NO_COLOR": {"NO_COLOR": "1"}, "json": {"STORE_OUTPUT": "json"}} {
		_, stdout, _ = runEnv(t, env, true, "version")
		if strings.Contains(stdout, "\x1b[") {
			t.Errorf("%s: unexpected color: %q", name, stdout)
		}
	}
}
