package ascclient_test

import (
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/nikeee/store-cli/internal/apple/asc"
	"github.com/nikeee/store-cli/internal/apple/ascclient"
	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/testutil/ascfake"
)

func TestAPIErrorMapping(t *testing.T) {
	tests := []struct {
		name string
		h    ascfake.HandlerFunc
		want clierr.Error
	}{
		{
			name: "fixture",
			h:    ascfake.JSON(http.StatusConflict, "error_409.json"),
			want: clierr.Error{
				Message: "An attribute value is invalid.",
				Status:  409,
				Code:    "ENTITY_ERROR.ATTRIBUTE.INVALID",
				Detail:  "The attribute 'whatsNew' can not be edited at this time.",
			},
		},
		{
			name: "several errors, no title",
			h: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusUnprocessableEntity)
				_, _ = io.WriteString(w, `{"errors":[{"status":"422","code":"A","detail":"first"},{"status":"422","code":"B","title":"T","detail":"second"}]}`)
			},
			want: clierr.Error{Message: "first", Status: 422, Code: "A", Detail: "first; second"},
		},
		{
			name: "not JSON",
			h: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/html")
				w.WriteHeader(http.StatusBadGateway)
				_, _ = io.WriteString(w, "<html><body>502 Bad Gateway</body></html>")
			},
			want: clierr.Error{Message: "app store connect: 502 Bad Gateway", Status: 502},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := ascfake.New(t)
			srv.Handle("GET", "/v1/apps", tt.h)
			_, err := newClient(t, srv, time.Now).AppsGetCollection(t.Context(), asc.AppsGetCollectionParams{})
			var re *asc.ResponseError
			if !errors.As(err, &re) {
				t.Fatalf("err = %v", err)
			}
			if got := ascclient.FromAPIError(re); *got != tt.want {
				t.Errorf("got  %+v\nwant %+v", *got, tt.want)
			}
		})
	}
}
