package cli_test

import (
	"encoding/json"
	"net/http"
	"slices"
	"testing"

	"github.com/nikeee/store-cli/internal/config"
	"github.com/nikeee/store-cli/internal/testutil/ascfake"
	"github.com/nikeee/store-cli/internal/testutil/golden"
)

func TestAppleAppList(t *testing.T) {
	srv := ascfake.New(t)
	page1, page2 := ascfake.JSON(http.StatusOK, "apps_page1.json"), ascfake.JSON(http.StatusOK, "apps_page2.json")
	srv.Handle("GET", "/v1/apps", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("cursor") != "" {
			page2(w, r)
			return
		}
		page1(w, r)
	})
	code, stdout, stderr := runEnv(t, appleEnv(srv, nil), false, "apple", "app", "list", "-o", "json")
	var apps []struct{ ID, BundleID, Name, SKU, PrimaryLocale string }
	if err := json.Unmarshal([]byte(stdout), &apps); err != nil || code != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q err=%v", code, stdout, stderr, err)
	}
	var ids []string
	for _, a := range apps {
		ids = append(ids, a.ID)
	}
	if want := []string{"1234567890", "1234567891", "1234567892"}; !slices.Equal(ids, want) || apps[2].BundleID != "de.example.beispiel" {
		t.Errorf("apps = %+v", apps)
	}
	if reqs := srv.Requests(); len(reqs) != 2 || reqs[0].Query.Get("limit") != "200" || reqs[1].Query.Get("cursor") != "Mg" {
		t.Errorf("requests = %+v", reqs)
	}
}

func TestAppleAppShow(t *testing.T) {
	for _, mode := range []string{"text", "json", "toon"} {
		t.Run(mode, func(t *testing.T) {
			srv := ascfake.New(t)
			srv.Handle("GET", "/v1/apps", ascfake.JSON(http.StatusOK, "apps_one.json"))
			srv.Handle("GET", "/v1/apps/{id}/appStoreVersions", ascfake.JSON(http.StatusOK, "app_appStoreVersions.json"))
			srv.Handle("GET", "/v1/apps/{id}/appInfos", ascfake.JSON(http.StatusOK, "app_appInfos.json"))
			srv.Handle("GET", "/v1/appInfos/{id}/appInfoLocalizations", ascfake.JSON(http.StatusOK, "appInfo_appInfoLocalizations.json"))
			// --app wins over the environment.
			env := appleEnv(srv, map[string]string{config.EnvAppleBundleID: "com.example.other"})
			args := []string{"apple", "app", "show", "-o", mode, "--app", "com.example.app"}
			if mode == "json" {
				env[config.EnvAppleBundleID] = "com.example.app"
				args = args[:5]
			}
			code, stdout, stderr := runEnv(t, env, false, args...)
			if code != 0 || stderr != "" {
				t.Fatalf("code=%d stderr=%q", code, stderr)
			}
			golden.Assert(t, "apple_app_show."+mode, []byte(stdout))
			if q := srv.Requests()[0].Query; q.Get("filter[bundleId]") != "com.example.app" {
				t.Errorf("resolve query = %s", q.Encode())
			}
		})
	}
}
