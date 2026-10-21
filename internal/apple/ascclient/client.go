// Package ascclient is the authenticated App Store Connect client: the
// generated operations of package asc plus JWT auth, pagination, error
// mapping, app lookup and presigned uploads.
package ascclient

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/nikeee/store-cli/internal/apple/asc"
	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/config"
	"github.com/nikeee/store-cli/internal/httpx"
)

// Client calls the App Store Connect API. The embedded asc.Client sends every
// request with a bearer token; how to call it: docs/decisions/0001-apple-codegen.md.
type Client struct {
	*asc.Client

	http *http.Client // without Authorization, for presigned upload URLs
}

// New returns a client for baseURL (asc.DefaultBaseURL in production) that
// sends requests through httpClient, the retrying client of package httpx.
// now is the clock for the token's iat and exp.
func New(creds config.AppleCredentials, httpClient *http.Client, baseURL string, now func() time.Time) (*Client, error) {
	tokens, err := newTokenSource(creds, now)
	if err != nil {
		return nil, err
	}
	return &Client{
		Client: asc.NewClient(baseURL, authDoer{http: httpClient, tokens: tokens}),
		http:   httpClient,
	}, nil
}

// authDoer adds the bearer token to every API request.
type authDoer struct {
	http   *http.Client
	tokens *tokenSource
}

func (d authDoer) Do(req *http.Request) (*http.Response, error) {
	tok, err := d.tokens.Token()
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := d.http.Do(req)
	return resp, httpx.RedactError(err)
}

// ResolveApp returns the app with bundleID.
func (c *Client) ResolveApp(ctx context.Context, bundleID string) (asc.App, error) {
	resp, err := c.AppsGetCollection(ctx, asc.AppsGetCollectionParams{FilterBundleID: []string{bundleID}})
	if err != nil {
		return asc.App{}, err
	}
	for _, a := range resp.Data {
		if a.Attributes != nil && a.Attributes.BundleID != nil && *a.Attributes.BundleID == bundleID {
			return a, nil
		}
	}
	return asc.App{}, &clierr.Error{
		Message: fmt.Sprintf("no app with bundle ID %q in App Store Connect", bundleID),
		Hint:    "store apple app list",
	}
}
