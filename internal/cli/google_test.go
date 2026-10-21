package cli_test

import (
	"encoding/json"
	"maps"
	"net/http"
	"strings"
	"testing"

	"github.com/nikeee/store-cli/internal/config"
	"github.com/nikeee/store-cli/internal/testutil/golden"
	"github.com/nikeee/store-cli/internal/testutil/playfake"
)

const googleEditsPath = "/androidpublisher/v3/applications/{packageName}/edits"

// googleEnv returns the environment for Google commands against srv, plus extra.
func googleEnv(srv *playfake.Server, extra map[string]string) map[string]string {
	env := map[string]string{
		config.EnvGoogleServiceAccount: string(playfake.ServiceAccountJSON),
		config.EnvInternalGoogleAPIURL: srv.URL,
	}
	maps.Copy(env, extra)
	return env
}

// googleEditServer returns a fake that inserts edit-1 and answers commit and delete.
func googleEditServer(t *testing.T) *playfake.Server {
	srv := playfake.New(t)
	srv.Handle("POST", googleEditsPath, playfake.JSON(http.StatusOK, "edit.json"))
	srv.Handle("POST", googleEditsPath+"/edit-1:commit", playfake.JSON(http.StatusOK, "edit.json"))
	srv.Handle("DELETE", googleEditsPath+"/edit-1", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	return srv
}

func TestGoogleAuthCheck(t *testing.T) {
	for _, mode := range []string{"text", "json", "toon"} {
		t.Run(mode, func(t *testing.T) {
			srv := googleEditServer(t)
			code, stdout, stderr := runEnv(t, googleEnv(srv, nil), false, "google", "auth", "check", "--app", "com.example.app", "-o", mode)
			if code != 0 || stderr != "" {
				t.Fatalf("code=%d stderr=%q", code, stderr)
			}
			golden.Assert(t, "google_auth_check."+mode, []byte(stdout))
			var calls []string
			for _, r := range srv.Requests() {
				calls = append(calls, r.Method+" "+r.Path)
			}
			if c := strings.Join(calls, ", "); c != "POST /androidpublisher/v3/applications/com.example.app/edits, DELETE /androidpublisher/v3/applications/com.example.app/edits/edit-1" {
				t.Errorf("calls = %s", c)
			}
		})
	}
}

func TestGoogleMissingAppExits2(t *testing.T) {
	srv := playfake.New(t)
	code, _, stderr := runEnv(t, googleEnv(srv, nil), false, "google", "auth", "check")
	if code != 2 || !strings.Contains(stderr, "--app") || !strings.Contains(stderr, config.EnvGooglePackageName) {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	if n := len(srv.Requests()); n != 0 {
		t.Errorf("%d requests sent", n)
	}
}

// The token request and the API requests go through the logging client; the
// log shows neither the token nor the JWT assertion.
func TestGoogleVerboseHidesToken(t *testing.T) {
	srv := googleEditServer(t)
	env := googleEnv(srv, map[string]string{config.EnvGooglePackageName: "com.example.app"})
	code, _, stderr := runEnv(t, env, false, "google", "auth", "check", "--verbose")
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stderr, "--> POST "+srv.URL+"/token\n") ||
		!strings.Contains(stderr, "--> POST "+srv.URL+"/androidpublisher/v3/applications/com.example.app/edits?") ||
		strings.Contains(stderr, playfake.AccessToken) || strings.Contains(stderr, "eyJ") || strings.Contains(stderr, "Bearer") {
		t.Errorf("stderr=%q", stderr)
	}
}

func TestGoogleAPIErrorPrinted(t *testing.T) {
	srv := playfake.New(t)
	srv.Handle("POST", googleEditsPath, playfake.JSON(http.StatusForbidden, "error_403.json"))
	code, stdout, stderr := runEnv(t, googleEnv(srv, nil), false, "google", "auth", "check", "--app", "com.example.app", "-o", "json")
	var got struct {
		Error, Code, Hint string
		Status            int
	}
	if err := json.Unmarshal([]byte(stderr), &got); err != nil || code != 1 || stdout != "" {
		t.Fatalf("code=%d stdout=%q stderr=%q err=%v", code, stdout, stderr, err)
	}
	if got.Error != "create edit: The caller does not have permission" || got.Status != 403 || got.Code != "PERMISSION_DENIED" ||
		got.Hint != "grant the service account access to the app in Play Console, Users and permissions" {
		t.Errorf("error = %+v", got)
	}
}
