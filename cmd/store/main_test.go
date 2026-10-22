package main

import (
	"bytes"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/nikeee/store-cli/internal/config"
	"github.com/nikeee/store-cli/internal/testutil/ascfake"
)

// runMainEnv makes the test binary run main instead of the tests.
const runMainEnv = "STORE_TEST_RUN_MAIN"

func TestMain(m *testing.M) {
	if os.Getenv(runMainEnv) == "1" {
		main()
		return
	}
	os.Exit(m.Run())
}

// CI runners stop jobs with SIGTERM: the command is cancelled and exits
// through its error path (exit 1) instead of being killed by the signal.
func TestSIGTERMCancelsTheCommand(t *testing.T) {
	srv := ascfake.New(t)
	srv.Handle("GET", "/v1/apps", ascfake.JSON(http.StatusOK, "apps_one.json"))
	srv.Handle("GET", "/v1/builds", ascfake.JSON(http.StatusOK, "builds_empty.json"))
	cmd := exec.Command(os.Args[0], "apple", "build", "wait", "45", "--app", "com.example.app", "-o", "text")
	cmd.Env = append(os.Environ(), runMainEnv+"=1",
		config.EnvAppleIssuerID+"="+ascfake.IssuerID,
		config.EnvAppleKeyID+"="+ascfake.KeyID,
		config.EnvApplePrivateKey+"="+string(ascfake.KeyPEM),
		config.EnvInternalAppleAPIURL+"="+srv.URL)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	// Signal once the command polls, i.e. sleeps until the next check.
	for deadline := time.Now().Add(10 * time.Second); !polled(srv); time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			_ = cmd.Process.Kill()
			t.Fatalf("command never polled; stderr = %s", stderr.String())
		}
	}
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	err := cmd.Wait()
	var ee *exec.ExitError
	if !errors.As(err, &ee) || ee.ExitCode() != 1 || !strings.Contains(stderr.String(), "context canceled") {
		t.Fatalf("err = %v, stderr = %s", err, stderr.String())
	}
}

func polled(srv *ascfake.Server) bool {
	for _, r := range srv.Requests() {
		if r.Path == "/v1/builds" {
			return true
		}
	}
	return false
}
