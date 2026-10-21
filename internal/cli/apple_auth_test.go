package cli_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/nikeee/store-cli/internal/testutil/ascfake"
	"github.com/nikeee/store-cli/internal/testutil/golden"
)

func TestAppleAuthCheck(t *testing.T) {
	for _, mode := range []string{"text", "json", "toon"} {
		t.Run(mode, func(t *testing.T) {
			srv := ascfake.New(t)
			srv.Handle("GET", "/v1/apps", ascfake.JSON(http.StatusOK, "apps_one.json"))
			code, stdout, stderr := runEnv(t, appleEnv(srv, nil), false, "apple", "auth", "check", "-o", mode)
			if code != 0 || stderr != "" {
				t.Fatalf("code=%d stderr=%q", code, stderr)
			}
			golden.Assert(t, "apple_auth_check."+mode, []byte(stdout))
			if q := srv.Requests()[0].Query.Encode(); q != "limit=1" {
				t.Errorf("query = %s", q)
			}
		})
	}
}

func TestAppleVerboseHidesToken(t *testing.T) {
	srv := ascfake.New(t)
	srv.Handle("GET", "/v1/apps", ascfake.JSON(http.StatusOK, "apps_one.json"))
	code, _, stderr := runEnv(t, appleEnv(srv, nil), false, "apple", "auth", "check", "--verbose")
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	token := strings.TrimPrefix(srv.Requests()[0].Header.Get("Authorization"), "Bearer ")
	if !strings.Contains(stderr, "--> GET "+srv.URL+"/v1/apps?limit=1") || strings.Contains(stderr, token) ||
		strings.Contains(stderr, "Bearer") || strings.Contains(stderr, "Authorization") {
		t.Errorf("stderr=%q", stderr)
	}
}

func TestAppleAPIErrorPrinted(t *testing.T) {
	srv := ascfake.New(t)
	srv.Handle("GET", "/v1/apps", ascfake.JSON(http.StatusUnauthorized, "error_401.json"))
	code, stdout, stderr := runEnv(t, appleEnv(srv, nil), false, "apple", "auth", "check", "-o", "json")
	var got struct {
		Error, Code, Detail string
		Status              int
	}
	if err := json.Unmarshal([]byte(stderr), &got); err != nil || code != 1 || stdout != "" {
		t.Fatalf("code=%d stdout=%q stderr=%q err=%v", code, stdout, stderr, err)
	}
	if got.Error != "Authentication credentials are missing or invalid." || got.Status != 401 || got.Code != "NOT_AUTHORIZED" ||
		!strings.HasPrefix(got.Detail, "Provide a properly configured and signed bearer token") {
		t.Errorf("error = %+v", got)
	}
}
