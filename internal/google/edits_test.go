package google

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/nikeee/store-cli/internal/output"
	"github.com/nikeee/store-cli/internal/testutil/playfake"
)

const (
	editsPath  = "/androidpublisher/v3/applications/{packageName}/edits"
	editPath   = editsPath + "/edit-1"
	commitPath = editsPath + "/edit-1:commit"
)

// newService returns a Service for com.example.app talking to srv; warnings
// go to stderr.
func newService(t *testing.T, srv *playfake.Server, stderr io.Writer) *Service {
	t.Helper()
	r := output.NewPrinter(output.ModeText, io.Discard, stderr, false)
	s, err := NewService(t.Context(), playfake.ServiceAccountJSON, &http.Client{}, srv.URL, "com.example.app", r)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// editServer returns a fake that inserts edit-1 and answers commit and delete.
func editServer(t *testing.T) *playfake.Server {
	srv := playfake.New(t)
	srv.Handle("POST", editsPath, playfake.JSON(http.StatusOK, "edit.json"))
	srv.Handle("POST", commitPath, playfake.JSON(http.StatusOK, "edit.json"))
	srv.Handle("DELETE", editPath, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	return srv
}

// calls lists method and path of every request srv received.
func calls(srv *playfake.Server) []string {
	var list []string
	for _, r := range srv.Requests() {
		list = append(list, r.Method+" "+strings.TrimPrefix(r.Path, "/androidpublisher/v3/applications/com.example.app"))
	}
	return list
}

func TestWithEditCommits(t *testing.T) {
	srv := editServer(t)
	var got string
	err := newService(t, srv, io.Discard).withEdit(t.Context(), CommitOptions{}, func(editID string) error {
		got = editID
		return nil
	})
	if err != nil || got != "edit-1" {
		t.Fatalf("err=%v editID=%q", err, got)
	}
	if c := strings.Join(calls(srv), ", "); c != "POST /edits, POST /edits/edit-1:commit" {
		t.Fatalf("calls = %s", c)
	}
	if q := srv.Requests()[1].Query; q.Has("changesNotSentForReview") {
		t.Errorf("commit query = %s", q.Encode())
	}
}

func TestWithEditDeletesOnError(t *testing.T) {
	errFn := errors.New("fn failed")
	tests := []struct {
		name       string
		commit     playfake.HandlerFunc
		deleteFail bool
		fnErr      error
		wantErr    string
		wantCalls  string
		wantWarn   string
	}{
		{name: "fn fails", fnErr: errFn, wantErr: "fn failed",
			wantCalls: "POST /edits, DELETE /edits/edit-1"},
		{name: "commit fails", commit: playfake.JSON(http.StatusBadRequest, "error_draft_app.json"),
			wantErr:   "commit edit: Only releases with status draft may be created on draft app.",
			wantCalls: "POST /edits, POST /edits/edit-1:commit, DELETE /edits/edit-1"},
		{name: "delete fails too", fnErr: errFn, deleteFail: true, wantErr: "fn failed",
			wantCalls: "POST /edits, DELETE /edits/edit-1",
			wantWarn:  "warning: could not delete edit edit-1: The caller does not have permission\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := editServer(t)
			if tt.commit != nil {
				srv.Handle("POST", commitPath, tt.commit)
			}
			if tt.deleteFail {
				srv.Handle("DELETE", editPath, playfake.JSON(http.StatusForbidden, "error_403.json"))
			}
			var stderr bytes.Buffer
			err := newService(t, srv, &stderr).withEdit(t.Context(), CommitOptions{}, func(string) error { return tt.fnErr })
			if err == nil || FromAPIError(err).Error() != tt.wantErr {
				t.Fatalf("err = %v", err)
			}
			if tt.fnErr != nil && !errors.Is(err, tt.fnErr) {
				t.Errorf("fn's error not returned: %v", err)
			}
			if c := strings.Join(calls(srv), ", "); c != tt.wantCalls {
				t.Errorf("calls = %s", c)
			}
			if stderr.String() != tt.wantWarn {
				t.Errorf("stderr = %q", stderr.String())
			}
		})
	}
}

func TestWithEditPassesChangesNotSentForReview(t *testing.T) {
	srv := editServer(t)
	if err := newService(t, srv, io.Discard).withEdit(t.Context(), CommitOptions{ChangesNotSentForReview: true}, func(string) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if c := strings.Join(calls(srv), ", "); c != "POST /edits, POST /edits/edit-1:commit" {
		t.Fatalf("calls = %s", c)
	}
	if q := srv.Requests()[1].Query.Get("changesNotSentForReview"); q != "true" {
		t.Errorf("commit query = %s", srv.Requests()[1].Query.Encode())
	}
}

func TestWithEditDryRunDeletes(t *testing.T) {
	srv := editServer(t)
	called := false
	err := newService(t, srv, io.Discard).withEdit(t.Context(), CommitOptions{DryRun: true}, func(string) error {
		called = true
		return nil
	})
	if err != nil || !called {
		t.Fatalf("err=%v called=%v", err, called)
	}
	if c := strings.Join(calls(srv), ", "); c != "POST /edits, DELETE /edits/edit-1" {
		t.Errorf("calls = %s", c)
	}
}

func TestCheckAuth(t *testing.T) {
	srv := editServer(t)
	s := newService(t, srv, io.Discard)
	if err := s.CheckAuth(t.Context()); err != nil {
		t.Fatal(err)
	}
	if c := strings.Join(calls(srv), ", "); c != "POST /edits, DELETE /edits/edit-1" {
		t.Errorf("calls = %s", c)
	}
	if s.ClientEmail() != playfake.ClientEmail || s.PackageName() != "com.example.app" {
		t.Errorf("clientEmail=%q packageName=%q", s.ClientEmail(), s.PackageName())
	}
}
