package ascclient_test

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
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
			want: clierr.Error{Message: "app store connect: 502 Bad Gateway", Status: 502, Detail: "<html><body>502 Bad Gateway</body></html>"},
		},
		{
			name: "long text",
			h: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = io.WriteString(w, "\n  "+strings.Repeat("x", 600))
			},
			want: clierr.Error{Message: "app store connect: 503 Service Unavailable", Status: 503, Detail: strings.Repeat("x", 500) + "..."},
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
			if got := ascclient.FromAPIError(re); !reflect.DeepEqual(*got, tt.want) {
				t.Errorf("got  %+v\nwant %+v", *got, tt.want)
			}
		})
	}
}

func TestMapErrorKeepsContext(t *testing.T) {
	re := &asc.ResponseError{StatusCode: 409, Response: asc.ErrorResponse{Errors: []asc.ErrorResponseErrorsItem{{
		Status: "409", Code: "ENTITY_ERROR.ATTRIBUTE.INVALID", Title: "An attribute value is invalid.",
		Detail: "The attribute 'whatsNew' can not be edited at this time.",
	}}}}
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"bare", re, "An attribute value is invalid."},
		{"wrapped", fmt.Errorf("attach build 42: %w", re), "attach build 42: An attribute value is invalid."},
		{"wrapped twice", fmt.Errorf("submit 1.2.0: %w", fmt.Errorf("attach build 42: %w", re)),
			"submit 1.2.0: attach build 42: An attribute value is invalid."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ce *clierr.Error
			if !errors.As(ascclient.MapError(tt.err), &ce) {
				t.Fatalf("not a clierr.Error: %v", ascclient.MapError(tt.err))
			}
			if ce.Message != tt.want || ce.Status != 409 || ce.Code != "ENTITY_ERROR.ATTRIBUTE.INVALID" ||
				ce.Detail != "The attribute 'whatsNew' can not be edited at this time." {
				t.Errorf("got %+v", *ce)
			}
		})
	}
	other := errors.New("boom")
	if got := ascclient.MapError(other); got.Error() != "boom" {
		t.Errorf("other error changed: %v", got)
	}
}
