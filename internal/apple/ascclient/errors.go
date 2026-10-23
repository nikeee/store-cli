package ascclient

import (
	"cmp"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/nikeee/store-cli/internal/apple/asc"
	"github.com/nikeee/store-cli/internal/clierr"
)

// FromAPIError turns an App Store Connect error response into a clierr.Error:
// Message is the first title (else its detail), Detail all details joined
// with "; ", Code the first code. A body that is not a JSON:API error
// document (e.g. a proxy's HTML page) gives the HTTP status, its start the
// detail.
func FromAPIError(e *asc.ResponseError) *clierr.Error {
	status := fmt.Sprintf("app store connect: %d %s", e.StatusCode, http.StatusText(e.StatusCode))
	errs := e.Response.Errors
	if len(errs) == 0 {
		return &clierr.Error{Message: status, Status: e.StatusCode, Detail: e.Body}
	}
	var details []string
	for _, x := range errs {
		if x.Detail != "" {
			details = append(details, x.Detail)
		}
	}
	return &clierr.Error{
		Message: cmp.Or(errs[0].Title, errs[0].Detail, status),
		Status:  e.StatusCode,
		Code:    errs[0].Code,
		Detail:  strings.Join(details, "; "),
	}
}

// MapError turns an App Store Connect error response anywhere in err's chain
// into a clierr.Error, keeping the context added by wrappers
// ("attach build 42: ..."); other errors pass unchanged.
func MapError(err error) error {
	var re *asc.ResponseError
	if !errors.As(err, &re) {
		return err
	}
	ce := FromAPIError(re)
	if prefix, ok := strings.CutSuffix(err.Error(), re.Error()); ok {
		ce.Message = prefix + ce.Message
	}
	return ce
}
