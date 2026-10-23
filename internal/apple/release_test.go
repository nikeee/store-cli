package apple

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/nikeee/store-cli/internal/apple/asc"
	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/testutil/ascfake"
)

const (
	phasedPath   = "/v1/appStoreVersions/{id}/appStoreVersionPhasedRelease"
	phasedID     = "4e6a8c20-1b3d-4f5a-9c7e-2d4f6a8b0c11"
	releasableID = "7d5f9a2e-3c41-4b8e-9f20-1a6b8c4d0130"
)

var (
	pendingV = Version{ID: releasableID, Platform: "ios", Version: "1.3.0", State: "PENDING_DEVELOPER_RELEASE"}
	liveV    = Version{ID: releasableID, Platform: "ios", Version: "1.3.0", State: "READY_FOR_DISTRIBUTION"}
)

func TestReleaseRequiresPendingDeveloperRelease(t *testing.T) {
	srv := ascfake.New(t)
	_, err := newService(t, srv).ReleaseVersion(t.Context(), editableV, false)
	if err == nil || !strings.Contains(err.Error(), "PREPARE_FOR_SUBMISSION") || clierr.ExitCode(err) != 1 {
		t.Fatalf("err = %v", err)
	}
	if len(srv.Requests()) != 0 {
		t.Errorf("requests = %v", requestLines(srv))
	}
}

func TestRelease(t *testing.T) {
	srv := ascfake.New(t)
	srv.Handle("POST", "/v1/appStoreVersionReleaseRequests", ascfake.JSON(http.StatusCreated, "release_request_created.json"))
	srv.Handle("GET", "/v1/appStoreVersions/{id}", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"type": "appStoreVersions", "id": releasableID,
			"attributes": map[string]any{"platform": "IOS", "versionString": "1.3.0", "appVersionState": "PROCESSING_FOR_DISTRIBUTION"}}, "links": map[string]any{"self": "x"}})
	})
	v, err := newService(t, srv).ReleaseVersion(t.Context(), pendingV, false)
	// the result is the version after the release, not as it was found
	if err != nil || v.Version != "1.3.0" || v.State != "PROCESSING_FOR_DISTRIBUTION" {
		t.Fatalf("v=%+v err=%v", v, err)
	}
	reqs := srv.Requests()
	if len(reqs) != 2 {
		t.Fatalf("requests = %v", requestLines(srv))
	}
	b := string(reqs[0].Body)
	if !strings.Contains(b, `"type":"appStoreVersionReleaseRequests"`) || !strings.Contains(b, `"appStoreVersion":{"data":{"type":"appStoreVersions","id":"`+releasableID+`"}}`) {
		t.Errorf("body = %s", b)
	}
}

func TestPhasedStartInactiveBeforeLive(t *testing.T) {
	srv := ascfake.New(t)
	srv.Handle("GET", phasedPath, ascfake.JSON(http.StatusOK, "phased_release_none.json"))
	srv.Handle("POST", "/v1/appStoreVersionPhasedReleases", ascfake.JSON(http.StatusCreated, "phased_release_created_inactive.json"))
	p, err := newService(t, srv).StartPhasedRelease(t.Context(), pendingV, false)
	if err != nil {
		t.Fatal(err)
	}
	if p != (PhasedRelease{Version: "1.3.0", State: "INACTIVE"}) {
		t.Errorf("p = %+v", p)
	}
	b := string(srv.Requests()[1].Body)
	if !strings.Contains(b, `"phasedReleaseState":"INACTIVE"`) || !strings.Contains(b, releasableID) {
		t.Errorf("body = %s", b)
	}
}

func TestPhasedStartActiveWhenLive(t *testing.T) {
	srv := ascfake.New(t)
	srv.Handle("GET", phasedPath, ascfake.JSON(http.StatusOK, "phased_release_none.json"))
	srv.Handle("POST", "/v1/appStoreVersionPhasedReleases", ascfake.JSON(http.StatusCreated, "phased_release_created_active.json"))
	p, err := newService(t, srv).StartPhasedRelease(t.Context(), liveV, false)
	if err != nil || p.State != "ACTIVE" || p.CurrentDayNumber != 1 {
		t.Fatalf("p=%+v err=%v", p, err)
	}
	if b := string(srv.Requests()[1].Body); !strings.Contains(b, `"phasedReleaseState":"ACTIVE"`) {
		t.Errorf("body = %s", b)
	}
}

func TestPhasedStartNotFoundMeansNone(t *testing.T) {
	srv := ascfake.New(t)
	srv.Handle("GET", phasedPath, ascfake.JSON(http.StatusNotFound, "phased_release_none.json"))
	srv.Handle("POST", "/v1/appStoreVersionPhasedReleases", ascfake.JSON(http.StatusCreated, "phased_release_created_inactive.json"))
	if _, err := newService(t, srv).StartPhasedRelease(t.Context(), pendingV, false); err != nil {
		t.Fatal(err)
	}
}

func TestPhasedStartIdempotent(t *testing.T) {
	srv := ascfake.New(t)
	srv.Handle("GET", phasedPath, ascfake.JSON(http.StatusOK, "phased_release_active.json"))
	p, err := newService(t, srv).StartPhasedRelease(t.Context(), liveV, false)
	if err != nil {
		t.Fatal(err)
	}
	if p != (PhasedRelease{Version: "1.3.0", State: "ACTIVE", CurrentDayNumber: 3}) {
		t.Errorf("p = %+v", p)
	}
	if len(srv.Requests()) != 1 {
		t.Errorf("requests = %v", requestLines(srv))
	}
}

func TestPhasedPause(t *testing.T) {
	srv := ascfake.New(t)
	srv.Handle("GET", phasedPath, ascfake.JSON(http.StatusOK, "phased_release_active.json"))
	srv.Handle("PATCH", "/v1/appStoreVersionPhasedReleases/{id}", ascfake.JSON(http.StatusOK, "phased_release_paused.json"))
	p, err := newService(t, srv).SetPhasedReleaseState(t.Context(), liveV, asc.PhasedReleaseStatePaused, false)
	if err != nil {
		t.Fatal(err)
	}
	if p != (PhasedRelease{Version: "1.3.0", State: "PAUSED", CurrentDayNumber: 3}) {
		t.Errorf("p = %+v", p)
	}
	reqs := srv.Requests()
	if reqs[1].Path != "/v1/appStoreVersionPhasedReleases/"+phasedID || !strings.Contains(string(reqs[1].Body), `"phasedReleaseState":"PAUSED"`) {
		t.Errorf("patch = %s %s", reqs[1].Path, reqs[1].Body)
	}
}

func TestPhasedPauseWithoutPhasedRelease(t *testing.T) {
	srv := ascfake.New(t)
	srv.Handle("GET", phasedPath, ascfake.JSON(http.StatusOK, "phased_release_none.json"))
	_, err := newService(t, srv).SetPhasedReleaseState(t.Context(), liveV, asc.PhasedReleaseStatePaused, false)
	var ce *clierr.Error
	if !errors.As(err, &ce) || ce.Hint != "store apple version phased start 1.3.0" || clierr.ExitCode(err) != 1 {
		t.Fatalf("err = %#v", err)
	}
	if len(srv.Requests()) != 1 {
		t.Errorf("requests = %v", requestLines(srv))
	}
}
