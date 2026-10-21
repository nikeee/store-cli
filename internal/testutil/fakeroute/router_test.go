package fakeroute_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nikeee/store-cli/internal/testutil/fakeroute"
)

// answer writes name and the editId path value.
func answer(name string) fakeroute.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, name+" "+r.PathValue("editId"))
	}
}

// call returns what the route matching method and path answers, "" for none.
func call(rt *fakeroute.Router, method, path string) string {
	r := httptest.NewRequest(method, path, nil)
	h := rt.Match(r)
	if h == nil {
		return ""
	}
	w := httptest.NewRecorder()
	h(w, r)
	return w.Body.String()
}

func TestRouterPlaceholderSuffix(t *testing.T) {
	var rt fakeroute.Router
	rt.Handle("POST", "/edits/{editId}:commit", answer("commit"))
	tests := []struct{ path, want string }{
		{"/edits/edit-1:commit", "commit edit-1"},
		{"/edits/edit-1:validate", ""},
		{"/edits/edit-1", ""},
		{"/edits/edit-1:commit/x", ""},
	}
	for _, tt := range tests {
		if got := call(&rt, "POST", tt.path); got != tt.want {
			t.Errorf("only :commit registered: POST %s = %q, want %q", tt.path, got, tt.want)
		}
	}

	rt.Handle("POST", "/edits/{editId}:validate", answer("validate"))
	rt.Handle("DELETE", "/edits/{editId}", answer("delete"))
	tests = []struct{ path, want string }{
		{"/edits/edit-1:commit", "commit edit-1"},
		{"/edits/edit-1:validate", "validate edit-1"},
	}
	for _, tt := range tests {
		if got := call(&rt, "POST", tt.path); got != tt.want {
			t.Errorf("POST %s = %q, want %q", tt.path, got, tt.want)
		}
	}
	// A plain placeholder takes the whole segment.
	if got := call(&rt, "DELETE", "/edits/edit-1:commit"); got != "delete edit-1:commit" {
		t.Errorf("plain placeholder: %q", got)
	}
	if got := rt.Registered(); got != "POST /edits/{editId}:commit, POST /edits/{editId}:validate, DELETE /edits/{editId}" {
		t.Errorf("registered = %s", got)
	}
}
