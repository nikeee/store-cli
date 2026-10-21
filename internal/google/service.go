// Package google implements the Google Play side of the CLI: one Service
// method per command, returning typed results.
package google

import (
	"context"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/androidpublisher/v3"
	"google.golang.org/api/option"

	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/output"
)

// Service runs Google Play Developer API operations for one app.
type Service struct {
	api         *androidpublisher.Service
	packageName string
	clientEmail string
	report      output.Reporter
}

// NewService returns a Service for the app packageName, authenticated as the
// service account of serviceAccountJSON. Token and API requests go through
// httpClient, the retrying client of package httpx. A non-empty baseURL
// replaces the API endpoint and the token URL (<baseURL>/token); tests point
// it at a fake. Progress and warnings go to r.
func NewService(ctx context.Context, serviceAccountJSON []byte, httpClient *http.Client, baseURL, packageName string, r output.Reporter) (*Service, error) {
	cfg, err := google.JWTConfigFromJSON(serviceAccountJSON, androidpublisher.AndroidpublisherScope)
	if err != nil {
		return nil, clierr.New("read service account key: %v", err)
	}
	opts := []option.ClientOption{}
	if baseURL != "" {
		cfg.TokenURL = baseURL + "/token"
		opts = append(opts, option.WithEndpoint(baseURL+"/"))
	}
	// The token source fetches tokens with the client in ctx.
	ctx = context.WithValue(ctx, oauth2.HTTPClient, httpClient)
	opts = append(opts, option.WithHTTPClient(cfg.Client(ctx)))
	api, err := androidpublisher.NewService(ctx, opts...)
	if err != nil {
		return nil, err
	}
	return &Service{api: api, packageName: packageName, clientEmail: cfg.Email, report: r}, nil
}

// ClientEmail is the service account the Service authenticates as.
func (s *Service) ClientEmail() string { return s.clientEmail }

// PackageName is the app the Service works on.
func (s *Service) PackageName() string { return s.packageName }
