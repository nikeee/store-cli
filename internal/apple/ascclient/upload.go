package ascclient

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/httpx"
)

const maxParallelUploads = 4

// UploadOperation is one part of an upload as App Store Connect describes it
// in uploadOperations: send file[Offset:Offset+Length] with Method to URL.
type UploadOperation struct {
	Method, URL    string
	Offset, Length int64
	Headers        map[string]string // from requestHeaders
}

// UploadParts performs ops, up to four at a time. The URLs are presigned, so
// the requests carry no Authorization header; errors never contain the URL's
// query.
func (c *Client) UploadParts(ctx context.Context, ops []UploadOperation, file io.ReaderAt) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var (
		wg       sync.WaitGroup
		once     sync.Once
		firstErr error
	)
	slots := make(chan struct{}, maxParallelUploads)
	for i, op := range ops {
		slots <- struct{}{}
		if ctx.Err() != nil {
			break
		}
		wg.Go(func() {
			defer func() { <-slots }()
			if err := c.uploadPart(ctx, op, file); err != nil {
				once.Do(func() {
					firstErr = fmt.Errorf("upload part %d of %d failed: %w", i+1, len(ops), err)
					cancel()
				})
			}
		})
	}
	wg.Wait()
	if firstErr != nil {
		return firstErr
	}
	return ctx.Err()
}

func (c *Client) uploadPart(ctx context.Context, op UploadOperation, file io.ReaderAt) error {
	body := func() io.ReadCloser { return io.NopCloser(io.NewSectionReader(file, op.Offset, op.Length)) }
	req, err := http.NewRequestWithContext(ctx, op.Method, op.URL, body())
	if err != nil {
		return httpx.RedactError(err)
	}
	req.ContentLength = op.Length
	req.GetBody = func() (io.ReadCloser, error) { return body(), nil } // lets httpx retry
	for k, v := range op.Headers {
		req.Header.Set(k, v)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return httpx.RedactError(err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return &clierr.Error{Message: resp.Status, Status: resp.StatusCode}
	}
	return nil
}
