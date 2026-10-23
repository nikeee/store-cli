package google

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/oauth2"
	"google.golang.org/api/googleapi"

	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/config"
	"github.com/nikeee/store-cli/internal/httpx"
)

// draftAppMessage is Play's error when a never published app gets a release
// that is not a draft.
const draftAppMessage = "Only releases with status draft may be created on draft app"

// FromAPIError turns a Google Play API error anywhere in err's chain into a
// clierr.Error, keeping the context added by wrappers before and after it
// ("commit edit: ...", "... (image set incomplete, ...)"), and hides the query
// values of a transport error's URL (upload sessions). Other errors pass
// unchanged.
func FromAPIError(err error) error {
	var re *oauth2.RetrieveError
	if errors.As(err, &re) {
		return fromTokenError(err, re)
	}
	var ge *googleapi.Error
	if errors.As(err, &ge) {
		ce := fromGoogleAPIError(ge)
		if before, after, ok := strings.Cut(err.Error(), ge.Error()); ok {
			ce.Message = before + ce.Message + after
		}
		return ce
	}
	var ue *url.Error
	if errors.As(err, &ue) {
		redacted := httpx.RedactError(ue)
		if before, after, ok := strings.Cut(err.Error(), ue.Error()); ok {
			return fmt.Errorf("%s%w%s", before, redacted, after)
		}
		return redacted
	}
	return err
}

// fromGoogleAPIError maps one API error: Message is Play's message (else the
// HTTP status), Code its status like "PERMISSION_DENIED", plus a hint for
// failures with a known fix.
func fromGoogleAPIError(e *googleapi.Error) *clierr.Error {
	var body struct {
		Error struct {
			Status string `json:"status"`
		} `json:"error"`
	}
	_ = json.Unmarshal([]byte(e.Body), &body)
	ce := &clierr.Error{
		Message: cmp.Or(e.Message, fmt.Sprintf("google play: %d %s", e.Code, http.StatusText(e.Code))),
		Status:  e.Code,
		Code:    body.Error.Status,
	}
	reasons := map[string]bool{}
	for _, it := range e.Errors {
		reasons[it.Reason] = true
	}
	switch {
	case strings.Contains(e.Message, draftAppMessage):
		ce.Hint = "use --status draft until the app is published"
	case reasons["accessNotConfigured"] || strings.Contains(e.Body, "SERVICE_DISABLED"):
		ce.Hint = "enable the Google Play Android Developer API in the Google Cloud project of the service account"
	case reasons["rateLimitExceeded"] || reasons["userRateLimitExceeded"] || reasons["quotaExceeded"] || e.Code == http.StatusTooManyRequests:
		ce.Hint = "Google Play limits requests per app; wait a minute and run the command again"
	case e.Code == http.StatusForbidden:
		ce.Hint = "grant the service account access to the app in Play Console, Users and permissions"
	}
	return ce
}

// fromTokenError maps a refused access token (re, somewhere in err): the key
// is deleted or revoked, or the clock is off. Context around it is kept.
func fromTokenError(err error, re *oauth2.RetrieveError) *clierr.Error {
	msg := "google play token: " + cmp.Or(re.ErrorCode, re.Error())
	if re.ErrorDescription != "" {
		msg += ": " + re.ErrorDescription
	}
	ce := &clierr.Error{Message: msg, Code: re.ErrorCode,
		Hint: "check " + config.EnvGoogleServiceAccount + ": the key may be deleted or revoked, or the clock of this machine is off"}
	var inner error = re
	var ue *url.Error
	if errors.As(err, &ue) {
		inner = ue
	}
	if before, after, ok := strings.Cut(err.Error(), inner.Error()); ok {
		ce.Message = before + ce.Message + after
	}
	return ce
}
