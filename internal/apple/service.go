// Package apple implements the App Store Connect side of the CLI: one Service
// method per command, returning typed results.
package apple

import (
	"context"
	"time"

	"github.com/nikeee/store-cli/internal/apple/asc"
	"github.com/nikeee/store-cli/internal/apple/ascclient"
	"github.com/nikeee/store-cli/internal/output"
)

// Service runs App Store Connect operations for the commands.
type Service struct {
	client *ascclient.Client
	report output.Reporter
	poll   Poll
	now    func() time.Time
}

// Poll configures waiting for App Store Connect processing.
type Poll struct {
	Interval time.Duration
	Sleep    func(ctx context.Context, d time.Duration) error
}

// NewService returns a Service using c; progress and warnings go to r.
func NewService(c *ascclient.Client, r output.Reporter, poll Poll, now func() time.Time) *Service {
	return &Service{client: c, report: r, poll: poll, now: now}
}

// App is an app in App Store Connect.
type App struct {
	ID            string `json:"id"`
	BundleID      string `json:"bundleId"`
	Name          string `json:"name"`
	SKU           string `json:"sku"`
	PrimaryLocale string `json:"primaryLocale"`
}

func appFromAPI(a asc.App) App {
	app := App{ID: a.ID}
	if at := a.Attributes; at != nil {
		app.BundleID = deref(at.BundleID)
		app.Name = deref(at.Name)
		app.SKU = deref(at.Sku)
		app.PrimaryLocale = deref(at.PrimaryLocale)
	}
	return app
}

// deref returns *p, or the zero value for nil (attributes the API omitted).
func deref[T any](p *T) T {
	if p == nil {
		var zero T
		return zero
	}
	return *p
}
