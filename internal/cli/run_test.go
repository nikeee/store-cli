package cli_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/nikeee/store-cli/internal/cli"
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
