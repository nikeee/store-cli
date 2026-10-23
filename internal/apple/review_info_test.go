package apple

import (
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/nikeee/store-cli/internal/apple/asc"
	"github.com/nikeee/store-cli/internal/metadata"
	"github.com/nikeee/store-cli/internal/testutil/ascfake"
)

var reviewVersion = Version{ID: "ver-1", Version: "1.3.0", Platform: "ios", State: "PREPARE_FOR_SUBMISSION"}

const reviewDetailJSON = `{"data":{"type":"appStoreReviewDetails","id":"rd-1","attributes":{"contactFirstName":"Ada","contactLastName":"Lovelace","contactPhone":"+1 555 0100","contactEmail":"ada@example.com","demoAccountRequired":true,"demoAccountName":"demo","demoAccountPassword":"s3cret","notes":"Use the demo."}},"links":{"self":"x"}}`

// reviewServer serves a review detail (none when detail is empty) and the
// given attachments.
func reviewServer(t *testing.T, detail string, attachments ...string) *ascfake.Server {
	t.Helper()
	srv := ascfake.New(t)
	srv.Handle("GET", "/v1/appStoreVersions/{id}/appStoreReviewDetail", func(w http.ResponseWriter, _ *http.Request) {
		if detail == "" {
			writeJSON(w, http.StatusOK, map[string]any{"data": nil, "links": map[string]any{"self": "x"}})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(detail))
	})
	srv.Handle("GET", "/v1/appStoreReviewDetails/{id}/appStoreReviewAttachments", func(w http.ResponseWriter, _ *http.Request) {
		data := []any{}
		for i, name := range attachments {
			data = append(data, map[string]any{"type": "appStoreReviewAttachments", "id": "att-" + string(rune('a'+i)), "attributes": map[string]any{"fileName": name}})
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": data, "links": map[string]any{"self": "x"}})
	})
	srv.Handle("POST", "/v1/appStoreReviewDetails", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusCreated, map[string]any{"data": map[string]any{"type": "appStoreReviewDetails", "id": "rd-new"}, "links": map[string]any{"self": "x"}})
	})
	srv.Handle("PATCH", "/v1/appStoreReviewDetails/{id}", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"type": "appStoreReviewDetails", "id": r.PathValue("id")}, "links": map[string]any{"self": "x"}})
	})
	return srv
}

func TestReviewInfoCreate(t *testing.T) {
	srv := reviewServer(t, "")
	texts, err := newService(t, srv).ReviewTexts(t.Context(), reviewVersion)
	if err != nil || texts != nil {
		t.Fatalf("texts=%v err=%v", texts, err)
	}
	changes := metadata.DiffTexts(metadata.LevelReview, "", metadata.AppleReviewFields, metadata.Texts{"contactFirstName": "Ada", "notes": "Hi", "contactPhone": ""}, texts)
	applied, err := newService(t, srv).ApplyReviewChanges(t.Context(), reviewVersion, changes)
	if err != nil || len(applied) != 2 {
		t.Fatalf("applied=%v err=%v", applied, err)
	}
	posts := requestsOf(srv, "POST")
	if len(posts) != 1 || len(requestsOf(srv, "PATCH")) != 0 {
		t.Fatalf("requests = %v", srv.Requests())
	}
	want := `{"data":{"type":"appStoreReviewDetails","attributes":{"contactFirstName":"Ada","notes":"Hi"},"relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"ver-1"}}}}}`
	if string(posts[0].Body) != want {
		t.Errorf("body = %s", posts[0].Body)
	}
}

func TestReviewInfoPatchOnlyChanged(t *testing.T) {
	srv := reviewServer(t, reviewDetailJSON)
	svc := newService(t, srv)
	remote, err := svc.ReviewTexts(t.Context(), reviewVersion)
	if err != nil {
		t.Fatal(err)
	}
	local := metadata.Texts{"contactFirstName": "Ada", "contactEmail": "new@example.com", "notes": ""}
	changes := metadata.DiffTexts(metadata.LevelReview, "", metadata.AppleReviewFields, local, remote)
	applied, err := svc.ApplyReviewChanges(t.Context(), reviewVersion, changes)
	if err != nil || len(applied) != 2 {
		t.Fatalf("applied=%v err=%v", applied, err)
	}
	patches := requestsOf(srv, "PATCH")
	if len(patches) != 1 || patches[0].Path != "/v1/appStoreReviewDetails/rd-1" || len(requestsOf(srv, "POST")) != 0 {
		t.Fatalf("requests = %v", srv.Requests())
	}
	// The unchanged first name is not sent; the empty notes are cleared with null.
	want := `{"data":{"type":"appStoreReviewDetails","id":"rd-1","attributes":{"contactEmail":"new@example.com","notes":null}}}`
	if string(patches[0].Body) != want {
		t.Errorf("body = %s", patches[0].Body)
	}
}

func TestReviewInfoDemoRequiredBool(t *testing.T) {
	srv := reviewServer(t, reviewDetailJSON)
	svc := newService(t, srv)
	_, changes, err := svc.SetReviewInfo(t.Context(), reviewVersion, metadata.Texts{"demoAccountRequired": "false"}, nil, false)
	if err != nil || len(changes) != 1 {
		t.Fatalf("changes=%v err=%v", changes, err)
	}
	patches := requestsOf(srv, "PATCH")
	want := `{"data":{"type":"appStoreReviewDetails","id":"rd-1","attributes":{"demoAccountRequired":false}}}`
	if len(patches) != 1 || string(patches[0].Body) != want {
		t.Fatalf("patches = %v", patches)
	}
	// "" is accepted and means no change: nothing is sent.
	_, changes, err = svc.SetReviewInfo(t.Context(), reviewVersion, metadata.Texts{"demoAccountRequired": ""}, nil, false)
	if err != nil || len(changes) != 0 || len(requestsOf(srv, "PATCH")) != 1 {
		t.Fatalf("changes=%v err=%v patches=%d", changes, err, len(requestsOf(srv, "PATCH")))
	}
	if _, _, err := svc.SetReviewInfo(t.Context(), reviewVersion, metadata.Texts{"demoAccountRequired": "yes"}, nil, false); err == nil {
		t.Error("yes accepted")
	}
}

func TestReviewInfoShowMasksPassword(t *testing.T) {
	svc := newService(t, reviewServer(t, reviewDetailJSON, "login.png"))
	masked, err := svc.ShowReviewInfo(t.Context(), reviewVersion, false)
	if err != nil {
		t.Fatal(err)
	}
	if masked.DemoPassword != "***" || masked.DemoUser != "demo" || !masked.DemoRequired || masked.FirstName != "Ada" ||
		masked.Phone != "+1 555 0100" || len(masked.Attachments) != 1 || masked.Attachments[0] != "login.png" {
		t.Errorf("masked = %+v", masked)
	}
	revealed, err := svc.ShowReviewInfo(t.Context(), reviewVersion, true)
	if err != nil || revealed.DemoPassword != "s3cret" {
		t.Errorf("revealed = %+v err=%v", revealed, err)
	}
	// An unset password stays empty instead of showing ***.
	none, err := newService(t, reviewServer(t, "")).ShowReviewInfo(t.Context(), reviewVersion, false)
	if err != nil || none.DemoPassword != "" || none.Attachments == nil {
		t.Errorf("none = %+v err=%v", none, err)
	}
}

func TestReviewInfoMappingCoversReviewFields(t *testing.T) {
	v := reflect.ValueOf(&ReviewInfo{}).Elem()
	for _, f := range metadata.AppleReviewFields {
		if _, ok := reviewFields[f.API]; !ok {
			t.Errorf("%s missing in reviewFields", f.API)
		}
		if !jsonFieldOf(v, camel(f.Flag)).IsValid() {
			t.Errorf("ReviewInfo has no field %s for %s", camel(f.Flag), f.File)
		}
	}
}

func attachmentFile(t *testing.T, name string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte("attachment bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func attachmentUploadServer(t *testing.T, detail string, existing ...string) *ascfake.Server {
	srv := reviewServer(t, detail, existing...)
	srv.Handle("DELETE", "/v1/appStoreReviewAttachments/{id}", ascfake.JSON(http.StatusNoContent, "empty.json"))
	srv.Handle("POST", "/v1/appStoreReviewAttachments", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusCreated, map[string]any{"data": map[string]any{"type": "appStoreReviewAttachments", "id": "att-new",
			"attributes": map[string]any{"uploadOperations": []any{map[string]any{"method": "PUT", "url": srv.UploadURL("att-new"), "offset": 0, "length": 16}}}},
			"links": map[string]any{"self": "x"}})
	})
	srv.Handle("PATCH", "/v1/appStoreReviewAttachments/{id}", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"type": "appStoreReviewAttachments", "id": r.PathValue("id")}, "links": map[string]any{"self": "x"}})
	})
	srv.Handle("GET", "/v1/appStoreReviewAttachments/{id}", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"type": "appStoreReviewAttachments", "id": r.PathValue("id"),
			"attributes": map[string]any{"assetDeliveryState": map[string]any{"state": "COMPLETE"}}}, "links": map[string]any{"self": "x"}})
	})
	return srv
}

func TestReviewInfoAttachmentsReplace(t *testing.T) {
	srv := attachmentUploadServer(t, reviewDetailJSON, "old1.png", "old2.png")
	file := attachmentFile(t, "login.txt")
	if err := newService(t, srv).ReplaceReviewAttachments(t.Context(), reviewVersion, []string{file}); err != nil {
		t.Fatal(err)
	}
	var seq []string
	for _, r := range srv.Requests() {
		if r.Method != "GET" {
			seq = append(seq, r.Method+" "+r.Path)
		}
	}
	got := strings.Join(seq, "\n")
	want := "DELETE /v1/appStoreReviewAttachments/att-a\nDELETE /v1/appStoreReviewAttachments/att-b\nPOST /v1/appStoreReviewAttachments\nPUT /upload/att-new\nPATCH /v1/appStoreReviewAttachments/att-new"
	if got != want {
		t.Errorf("requests:\n%s\nwant:\n%s", got, want)
	}
	post := requestsOf(srv, "POST")[0]
	if !strings.Contains(string(post.Body), `"fileName":"login.txt"`) || !strings.Contains(string(post.Body), `"id":"rd-1"`) {
		t.Errorf("post body = %s", post.Body)
	}
	patch := requestsOf(srv, "PATCH")[0]
	if !strings.Contains(string(patch.Body), `"uploaded":true`) || !strings.Contains(string(patch.Body), `"sourceFileChecksum":"`) {
		t.Errorf("patch body = %s", patch.Body)
	}
}

func TestReviewInfoAttachmentsCreateDetail(t *testing.T) {
	srv := attachmentUploadServer(t, "")
	if err := newService(t, srv).ReplaceReviewAttachments(t.Context(), reviewVersion, []string{attachmentFile(t, "a.txt")}); err != nil {
		t.Fatal(err)
	}
	if posts := requestsOf(srv, "POST"); len(posts) != 2 || posts[0].Path != "/v1/appStoreReviewDetails" || !strings.Contains(string(posts[1].Body), `"id":"rd-new"`) {
		t.Errorf("posts = %v", posts)
	}
}

func TestReviewInfoDryRunSendsOnlyGets(t *testing.T) {
	srv := attachmentUploadServer(t, reviewDetailJSON, "old.png")
	info, changes, err := newService(t, srv).SetReviewInfo(t.Context(), reviewVersion,
		metadata.Texts{"contactFirstName": "Grace", "demoAccountPassword": "newpass"}, []string{attachmentFile(t, "a.txt")}, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range srv.Requests() {
		if r.Method != "GET" {
			t.Errorf("non-GET request %s %s", r.Method, r.Path)
		}
	}
	if len(changes) != 3 || changes[1].Old != "***" || changes[1].New != "***" || changes[2].Field != "attachments" || changes[2].New != "a.txt" {
		t.Errorf("changes = %+v", changes)
	}
	if info.FirstName != "Grace" || info.DemoPassword != "***" || len(info.Attachments) != 1 || info.Attachments[0] != "a.txt" {
		t.Errorf("info = %+v", info)
	}
}

func TestReviewInfoMissingAttachmentSendsNothing(t *testing.T) {
	srv := reviewServer(t, reviewDetailJSON)
	_, _, err := newService(t, srv).SetReviewInfo(t.Context(), reviewVersion, metadata.Texts{"notes": "x"}, []string{filepath.Join(t.TempDir(), "nope.png")}, false)
	if err == nil || len(srv.Requests()) != 0 {
		t.Fatalf("err=%v requests=%v", err, srv.Requests())
	}
}

func TestReviewInfoApplyReturnsMaskedChanges(t *testing.T) {
	srv := reviewServer(t, reviewDetailJSON)
	changes := []metadata.Change{{Action: metadata.ActionUpdate, Scope: "review", Field: "demoAccountPassword", Old: "s3cret", New: "newpass"}}
	applied, err := newService(t, srv).ApplyReviewChanges(t.Context(), reviewVersion, changes)
	if err != nil || len(applied) != 1 || applied[0].New != "***" || applied[0].Old != "***" {
		t.Fatalf("applied=%+v err=%v", applied, err)
	}
	if patches := requestsOf(srv, "PATCH"); len(patches) != 1 || !strings.Contains(string(patches[0].Body), `"demoAccountPassword":"newpass"`) {
		t.Errorf("patches = %v", patches)
	}
}

func TestReviewInfoSetReturnsDetailReadError(t *testing.T) {
	srv := reviewServer(t, reviewDetailJSON)
	// the attachments are read after the PATCH; that read fails
	srv.Handle("GET", "/v1/appStoreReviewDetails/{id}/appStoreReviewAttachments", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"errors": []any{map[string]any{"status": "500", "code": "UNEXPECTED_ERROR", "title": "boom"}}})
	})
	_, _, err := newService(t, srv).SetReviewInfo(t.Context(), reviewVersion, metadata.Texts{"notes": "changed"}, nil, false)
	if err == nil {
		t.Fatal("error swallowed")
	}
	if len(requestsOf(srv, "PATCH")) != 1 {
		t.Errorf("requests = %v", srv.Requests())
	}
}

// SetReviewInfo reads the review detail once, also when it changes texts and
// attachments.
func TestReviewInfoSetReadsDetailOnce(t *testing.T) {
	srv := attachmentUploadServer(t, reviewDetailJSON, "old.png")
	_, _, err := newService(t, srv).SetReviewInfo(t.Context(), reviewVersion, metadata.Texts{"notes": "changed"}, []string{attachmentFile(t, "a.txt")}, false)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, r := range requestsOf(srv, "GET") {
		if strings.HasSuffix(r.Path, "/appStoreReviewDetail") {
			n++
		}
	}
	if n != 1 {
		t.Errorf("review detail read %d times", n)
	}
}

// An attachment without attributes still counts, under its ID.
func TestAttachmentNamesKeepsBareAttachments(t *testing.T) {
	got := attachmentNames([]asc.AppStoreReviewAttachment{{ID: "att-1", Attributes: &asc.AppStoreReviewAttachmentAttributes{FileName: new("a.png")}}, {ID: "att-2"}})
	if !slices.Equal(got, []string{"a.png", "att-2"}) {
		t.Errorf("names = %v", got)
	}
}
