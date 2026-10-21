package google

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"google.golang.org/api/googleapi"

	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/httpx"
)

// draftAppMessage is Play's error when a never published app gets a release
// that is not a draft.
const draftAppMessage = "Only releases with status draft may be created on draft app"

// FromAPIError turns a Google Play API error anywhere in err's chain into a
// clierr.Error, keeping the context added by wrappers ("commit edit: ..."),
// and hides the query values of a transport error's URL (upload sessions).
// Other errors pass unchanged.
func FromAPIError(err error) error {
	var ge *googleapi.Error
	if errors.As(err, &ge) {
		ce := fromGoogleAPIError(ge)
		if prefix, ok := strings.CutSuffix(err.Error(), ge.Error()); ok {
			ce.Message = prefix + ce.Message
		}
		return ce
	}
	var ue *url.Error
	if errors.As(err, &ue) {
		redacted := httpx.RedactError(ue)
		if prefix, ok := strings.CutSuffix(err.Error(), ue.Error()); ok {
			return fmt.Errorf("%s%w", prefix, redacted)
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
	switch {
	case strings.Contains(e.Message, draftAppMessage):
		ce.Hint = "use --status draft until the app is published"
	case e.Code == http.StatusForbidden:
		ce.Hint = "grant the service account access to the app in Play Console, Users and permissions"
	}
	return ce
}
