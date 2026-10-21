package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/nikeee/store-cli/internal/apple/asc"
	"github.com/nikeee/store-cli/internal/clierr"
)

func TestAppleAPIErrorKeepsContext(t *testing.T) {
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
			if !errors.As(appleAPIError(tt.err), &ce) {
				t.Fatalf("not a clierr.Error: %v", appleAPIError(tt.err))
			}
			if ce.Message != tt.want || ce.Status != 409 || ce.Code != "ENTITY_ERROR.ATTRIBUTE.INVALID" ||
				ce.Detail != "The attribute 'whatsNew' can not be edited at this time." {
				t.Errorf("got %+v", *ce)
			}
		})
	}
	other := errors.New("boom")
	if got := appleAPIError(other); got.Error() != "boom" {
		t.Errorf("other error changed: %v", got)
	}
}
