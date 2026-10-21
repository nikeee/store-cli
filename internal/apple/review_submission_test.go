package apple

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/nikeee/store-cli/internal/apple/ascclient"
	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/output"
	"github.com/nikeee/store-cli/internal/testutil/ascfake"
)

const (
	submissionID = "5c1b7e30-0a42-4d9f-8e11-3b6f2a9d7c01"
	submitPath   = "/v1/reviewSubmissions/{id}"
)

var editableV = Version{ID: "7d5f9a2e-3c41-4b8e-9f20-1a6b8c4d0130", Platform: "ios", Version: "1.3.0", State: "PREPARE_FOR_SUBMISSION"}

func submissionServer(t *testing.T, list string) *ascfake.Server {
	t.Helper()
	srv := ascfake.New(t)
	srv.Handle("GET", "/v1/apps/{id}/reviewSubmissions", ascfake.JSON(http.StatusOK, list))
	srv.Handle("POST", "/v1/reviewSubmissions", ascfake.JSON(http.StatusCreated, "review_submission_created.json"))
	srv.Handle("POST", "/v1/reviewSubmissionItems", ascfake.JSON(http.StatusCreated, "review_submission_item_created.json"))
	srv.Handle("PATCH", submitPath, ascfake.JSON(http.StatusOK, "review_submission_submitted.json"))
	return srv
}

func requestLines(srv *ascfake.Server) []string {
	var out []string
	for _, r := range srv.Requests() {
		out = append(out, r.Method+" "+r.Path)
	}
	return out
}

func TestSubmitCreatesSubmission(t *testing.T) {
	srv := submissionServer(t, "review_submissions_none.json")
	res, err := newService(t, srv).SubmitVersion(t.Context(), versionsApp, editableV)
	if err != nil {
		t.Fatal(err)
	}
	if res != (SubmissionResult{Version: "1.3.0", SubmissionID: submissionID, State: "WAITING_FOR_REVIEW", Items: 1}) {
		t.Errorf("res = %+v", res)
	}
	want := []string{
		"GET /v1/apps/1234567890/reviewSubmissions",
		"POST /v1/reviewSubmissions",
		"POST /v1/reviewSubmissionItems",
		"PATCH /v1/reviewSubmissions/" + submissionID,
	}
	if got := requestLines(srv); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("requests = %v", got)
	}
	reqs := srv.Requests()
	q := reqs[0].Query
	if q.Get("filter[platform]") != "IOS" || q.Get("filter[state]") != "READY_FOR_REVIEW" || q.Get("include") != "items,appStoreVersionForReview" {
		t.Errorf("query = %s", q.Encode())
	}
	if b := string(reqs[1].Body); !strings.Contains(b, `"platform":"IOS"`) || !strings.Contains(b, `"id":"1234567890"`) {
		t.Errorf("create body = %s", b)
	}
	if b := string(reqs[2].Body); !strings.Contains(b, `"id":"`+submissionID+`"`) || !strings.Contains(b, `"appStoreVersion"`) || !strings.Contains(b, editableV.ID) {
		t.Errorf("item body = %s", b)
	}
	if b := string(reqs[3].Body); !strings.Contains(b, `"submitted":true`) {
		t.Errorf("patch body = %s", b)
	}
}

func TestSubmitReusesDraftWithSubscription(t *testing.T) {
	srv := submissionServer(t, "review_submissions_draft.json")
	c, err := ascclient.New(ascfake.Credentials(), &http.Client{}, srv.URL, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	var progress bytes.Buffer
	svc := NewService(c, output.NewPrinter(output.ModeText, io.Discard, &progress, false), Poll{}, time.Now)
	res, err := svc.SubmitVersion(t.Context(), versionsApp, editableV)
	if err != nil {
		t.Fatal(err)
	}
	if res.SubmissionID != submissionID || res.Items != 2 {
		t.Errorf("res = %+v", res)
	}
	if !strings.Contains(progress.String(), "reusing the open review submission with 1 item(s)") {
		t.Errorf("progress = %q", progress.String())
	}
	want := "GET /v1/apps/1234567890/reviewSubmissions\nPOST /v1/reviewSubmissionItems\nPATCH /v1/reviewSubmissions/" + submissionID
	if got := strings.Join(requestLines(srv), "\n"); got != want {
		t.Errorf("requests = %s", got)
	}
}

func TestSubmitDraftHoldsOtherVersion(t *testing.T) {
	srv := submissionServer(t, "review_submissions_draft_other_version.json")
	_, err := newService(t, srv).SubmitVersion(t.Context(), versionsApp, editableV)
	if err == nil || !strings.Contains(err.Error(), "1.4.0") {
		t.Fatalf("err = %v", err)
	}
	if clierr.ExitCode(err) != 1 || len(srv.Requests()) != 1 {
		t.Errorf("code=%d requests=%v", clierr.ExitCode(err), requestLines(srv))
	}
}

func TestSubmitNotEditable(t *testing.T) {
	for state, hint := range map[string]string{
		"WAITING_FOR_REVIEW":     "store apple version cancel 1.2.0",
		"IN_REVIEW":              "store apple version cancel 1.2.0",
		"READY_FOR_DISTRIBUTION": "",
	} {
		srv := submissionServer(t, "review_submissions_none.json")
		v := Version{ID: "x", Platform: "ios", Version: "1.2.0", State: state}
		_, err := newService(t, srv).SubmitVersion(t.Context(), versionsApp, v)
		var ce *clierr.Error
		if !errors.As(err, &ce) || ce.Code != "STATE_ERROR" || ce.Hint != hint || clierr.ExitCode(err) != 1 ||
			ce.Message != "version 1.2.0 is in state "+state+", nothing to submit" {
			t.Fatalf("%s: err = %#v", state, err)
		}
		if len(srv.Requests()) != 0 {
			t.Errorf("%s: requests = %v", state, requestLines(srv))
		}
	}
}

func TestSubmitDraftAlreadyHoldsVersion(t *testing.T) {
	srv := submissionServer(t, "review_submissions_draft_holds_version.json")
	res, err := newService(t, srv).SubmitVersion(t.Context(), versionsApp, editableV)
	if err != nil {
		t.Fatal(err)
	}
	if res.Items != 2 {
		t.Errorf("res = %+v", res)
	}
	want := "GET /v1/apps/1234567890/reviewSubmissions\nPATCH /v1/reviewSubmissions/" + submissionID
	if got := strings.Join(requestLines(srv), "\n"); got != want {
		t.Errorf("requests = %s", got)
	}
}

func TestPreviewSubmitVersion(t *testing.T) {
	srv := submissionServer(t, "review_submissions_draft.json")
	res, err := newService(t, srv).PreviewSubmitVersion(t.Context(), versionsApp, editableV)
	if err != nil || res != (SubmissionResult{Version: "1.3.0", SubmissionID: submissionID, State: "dry-run", Items: 2}) {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if len(srv.Requests()) != 1 {
		t.Errorf("requests = %v", requestLines(srv))
	}
}

func TestCancel(t *testing.T) {
	srv := submissionServer(t, "review_submissions_waiting.json")
	srv.Handle("PATCH", submitPath, ascfake.JSON(http.StatusOK, "review_submission_canceled.json"))
	v := editableV
	v.State = "WAITING_FOR_REVIEW"
	res, err := newService(t, srv).CancelVersion(t.Context(), versionsApp, v)
	if err != nil {
		t.Fatal(err)
	}
	if res.SubmissionID != submissionID || res.State != "CANCELING" || res.Version != "1.3.0" {
		t.Errorf("res = %+v", res)
	}
	reqs := srv.Requests()
	if q := reqs[0].Query; q.Get("filter[state]") != "WAITING_FOR_REVIEW,IN_REVIEW,UNRESOLVED_ISSUES" || q.Get("filter[platform]") != "IOS" {
		t.Errorf("query = %s", q.Encode())
	}
	if b := string(reqs[1].Body); reqs[1].Method != "PATCH" || !strings.Contains(b, `"canceled":true`) {
		t.Errorf("request = %s %s", reqs[1].Method, b)
	}
}

func TestCancelNoSubmission(t *testing.T) {
	srv := submissionServer(t, "review_submissions_none.json")
	_, err := newService(t, srv).CancelVersion(t.Context(), versionsApp, editableV)
	if err == nil || clierr.ExitCode(err) != 1 || len(srv.Requests()) != 1 {
		t.Fatalf("err=%v requests=%v", err, requestLines(srv))
	}
}

func TestReviewSubmissionsCountItemsPerSubmission(t *testing.T) {
	srv := submissionServer(t, "review_submissions_two.json")
	subs, err := newService(t, srv).listReviewSubmissions(t.Context(), versionsApp, "ios", []string{"READY_FOR_REVIEW", "WAITING_FOR_REVIEW"})
	if err != nil || len(subs) != 2 {
		t.Fatalf("subs=%+v err=%v", subs, err)
	}
	if subs[0].items != 2 || subs[1].items != 0 {
		t.Errorf("items = %d and %d, want 2 and 0", subs[0].items, subs[1].items)
	}
}
