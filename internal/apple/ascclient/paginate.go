package ascclient

import (
	"context"

	"github.com/nikeee/store-cli/internal/apple/asc"
)

// Paginate walks a collection from its first page: it calls page with first
// and then with every page behind links.next. page collects the page's items
// and returns its links.next.
//
//	first, err := c.AppsGetCollection(ctx, asc.AppsGetCollectionParams{Limit: 200})
//	if err != nil {
//		return err
//	}
//	var apps []asc.App
//	err = ascclient.Paginate(ctx, c, first, func(p *asc.AppsResponse) *string {
//		apps = append(apps, p.Data...)
//		return p.Links.Next
//	})
func Paginate[T any](ctx context.Context, c *Client, first *T, page func(*T) *string) error {
	for p := first; ; {
		next := page(p)
		if next == nil {
			return nil
		}
		var err error
		if p, err = asc.GetNext[T](ctx, c.Client, *next); err != nil {
			return err
		}
	}
}
