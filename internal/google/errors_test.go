package google

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"testing"

	"golang.org/x/oauth2"
	"google.golang.org/api/googleapi"

	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/testutil/playfake"
)

func TestGoogleErrorHints(t *testing.T) {
	tests := []struct {
		name string
		h    playfake.HandlerFunc
		want clierr.Error
	}{
		{
			name: "draft app",
			h:    playfake.JSON(http.StatusBadRequest, "error_draft_app.json"),
			want: clierr.Error{
				Message: "create edit: Only releases with status draft may be created on draft app.",
				Status:  400,
				Code:    "INVALID_ARGUMENT",
				Hint:    "use --status draft until the app is published",
			},
		},
		{
			name: "no access",
			h:    playfake.JSON(http.StatusForbidden, "error_403.json"),
			want: clierr.Error{
				Message: "create edit: The caller does not have permission",
				Status:  403,
				Code:    "PERMISSION_DENIED",
				Hint:    "grant the service account access to the app in Play Console, Users and permissions",
			},
		},
		{
			name: "not JSON",
			h: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/html")
				w.WriteHeader(http.StatusBadGateway)
				_, _ = io.WriteString(w, "<html><body>502 Bad Gateway</body></html>")
			},
			want: clierr.Error{Message: "create edit: google play: 502 Bad Gateway", Status: 502},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := playfake.New(t)
			srv.Handle("POST", editsPath, tt.h)
			err := newService(t, srv, io.Discard).withEdit(t.Context(), CommitOptions{}, func(string) error { return nil })
			var ce *clierr.Error
			if !errors.As(FromAPIError(err), &ce) {
				t.Fatalf("not a clierr.Error: %v", err)
			}
			if !reflect.DeepEqual(*ce, tt.want) {
				t.Errorf("got  %+v\nwant %+v", *ce, tt.want)
			}
		})
	}
	other := errors.New("boom")
	if got := FromAPIError(other); got.Error() != "boom" {
		t.Errorf("other error changed: %v", got)
	}
}

func TestFromAPIErrorRedactsTransportErrors(t *testing.T) {
	ue := &url.Error{Op: "Post", URL: "https://example.com/upload/x?uploadType=resumable&upload_id=secret", Err: errors.New("connection reset")}
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"bare", ue, `Post "https://example.com/upload/x?uploadType=resumable&upload_id=***": connection reset`},
		{"wrapped", fmt.Errorf("upload bundle: %w", ue), `upload bundle: Post "https://example.com/upload/x?uploadType=resumable&upload_id=***": connection reset`},
		{"context after", fmt.Errorf("en-US phone: %w (image set incomplete)", ue),
			`en-US phone: Post "https://example.com/upload/x?uploadType=resumable&upload_id=***": connection reset (image set incomplete)`},
	}
	for _, tt := range tests {
		if got := FromAPIError(tt.err).Error(); got != tt.want {
			t.Errorf("%s: got %q", tt.name, got)
		}
	}
}

// Context a wrapper adds after the API error, like the "image set incomplete"
// note of SyncImages, is kept.
func TestFromAPIErrorKeepsContextAfter(t *testing.T) {
	ge := &googleapi.Error{Code: http.StatusBadRequest, Message: "Invalid request", Body: `{"error":{"status":"INVALID_ARGUMENT"}}`}
	err := fmt.Errorf("staged 1 of 2 changes: en-US phone: upload a.png: %w (image set incomplete, run again)", ge)
	var ce *clierr.Error
	if !errors.As(FromAPIError(err), &ce) {
		t.Fatalf("not a clierr.Error: %v", err)
	}
	if want := "staged 1 of 2 changes: en-US phone: upload a.png: Invalid request (image set incomplete, run again)"; ce.Message != want || ce.Code != "INVALID_ARGUMENT" {
		t.Errorf("got %+v", *ce)
	}
}

// A 403 gets the hint that fits its reason.
func TestGoogle403HintByReason(t *testing.T) {
	tests := []struct{ name, body, hint string }{
		{"no access", `{"error":{"code":403,"message":"The caller does not have permission","status":"PERMISSION_DENIED"}}`,
			"grant the service account access to the app in Play Console, Users and permissions"},
		{"api disabled", `{"error":{"code":403,"message":"Google Play Android Developer API has not been used in project 1 before or it is disabled.",` +
			`"errors":[{"reason":"accessNotConfigured"}],"status":"PERMISSION_DENIED"}}`,
			"enable the Google Play Android Developer API in the Google Cloud project of the service account"},
		{"rate limit", `{"error":{"code":403,"message":"Rate Limit Exceeded","errors":[{"reason":"rateLimitExceeded"}]}}`,
			"Google Play limits requests per app; wait a minute and run the command again"},
	}
	for _, tt := range tests {
		ge := &googleapi.Error{Code: http.StatusForbidden, Body: tt.body}
		_ = json.Unmarshal([]byte(tt.body), &struct {
			Error *googleapi.Error `json:"error"`
		}{ge})
		if got := fromGoogleAPIError(ge).Hint; got != tt.hint {
			t.Errorf("%s: hint = %q", tt.name, got)
		}
	}
}

// A token the account server refuses (revoked key, wrong clock) names the
// credentials.
func TestGoogleTokenErrorHint(t *testing.T) {
	re := &oauth2.RetrieveError{ErrorCode: "invalid_grant", ErrorDescription: "Invalid JWT Signature."}
	err := &url.Error{Op: "Post", URL: "https://oauth2.googleapis.com/token", Err: fmt.Errorf("oauth2: cannot fetch token: %w", re)}
	var ce *clierr.Error
	if !errors.As(FromAPIError(fmt.Errorf("create edit: %w", err)), &ce) {
		t.Fatalf("not a clierr.Error: %v", err)
	}
	if ce.Code != "invalid_grant" || ce.Hint != "check GOOGLE_PLAY_SERVICE_ACCOUNT_JSON: the key may be deleted or revoked, or the clock of this machine is off" ||
		ce.Message != "create edit: google play token: invalid_grant: Invalid JWT Signature." {
		t.Errorf("got %+v", *ce)
	}
}
