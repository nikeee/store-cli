package apple

import (
	"context"
	"crypto/md5" //nolint:gosec // App Store Connect identifies uploads by MD5.
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nikeee/store-cli/internal/apple/asc"
	"github.com/nikeee/store-cli/internal/apple/ascclient"
	"github.com/nikeee/store-cli/internal/clierr"
)

// defaultAssetTimeout is how long one file may take from reservation until
// Apple finished processing it.
const defaultAssetTimeout = 10 * time.Minute

// assetKind is what differs between screenshots, app previews and App Review
// attachments. All three are reserved with a create request that returns
// upload operations, committed with a PATCH (uploaded: true plus the MD5), and
// processed asynchronously (assetDeliveryState). uploadAsset runs that flow
// for any kind; previews and attachments add their own kind next to
// screenshotKind.
type assetKind struct {
	// name is used in messages: "screenshot", "preview", "attachment".
	name string
	// reserve creates the asset below parentID and returns its upload operations.
	reserve func(ctx context.Context, c *ascclient.Client, parentID, fileName string, size int64) (assetReservation, error)
	// commit marks the asset as uploaded and sends the MD5 of the file (hex).
	commit func(ctx context.Context, c *ascclient.Client, id, md5 string) error
	// state reads the delivery state of the asset.
	state func(ctx context.Context, c *ascclient.Client, id string) (assetState, error)
}

type assetReservation struct {
	ID  string
	Ops []asc.UploadOperation
}

// assetState is assetDeliveryState: State is AWAITING_UPLOAD, UPLOAD_COMPLETE,
// COMPLETE or FAILED (empty when Apple sent none).
type assetState struct {
	State    string
	Errors   []asc.AppMediaStateError
	Warnings []asc.AppMediaStateError
}

// uploadAsset reserves, uploads, commits and waits for one file and returns
// the ID of the asset. timeout bounds all of that for this file. Progress goes
// to stderr.
func (s *Service) uploadAsset(ctx context.Context, kind assetKind, parentID, path string, timeout time.Duration) (string, error) {
	name := filepath.Base(path)
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	fi, err := f.Stat()
	if err != nil {
		return "", err
	}
	h := md5.New() //nolint:gosec
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	sum := hex.EncodeToString(h.Sum(nil))

	parent := ctx
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	doing := "reserving " + kind.name + " " + name
	id, err := s.runAsset(ctx, kind, parentID, name, f, fi.Size(), sum, timeout, &doing)
	if err != nil && parent.Err() == nil && errors.Is(err, context.DeadlineExceeded) {
		return id, clierr.New("timed out after %s while %s", timeout, doing)
	}
	return id, err
}

func (s *Service) runAsset(ctx context.Context, kind assetKind, parentID, name string, f io.ReaderAt, size int64, sum string, timeout time.Duration, doing *string) (string, error) {
	start := s.now()
	res, err := kind.reserve(ctx, s.client, parentID, name, size)
	if err != nil {
		return "", fmt.Errorf("reserve %s %s: %w", kind.name, name, err)
	}
	ops, err := assetOperations(res.Ops)
	if err != nil {
		return res.ID, fmt.Errorf("%s %s: %w", kind.name, name, err)
	}

	*doing = "uploading " + kind.name + " " + name
	s.report.Progressf("uploading %s %s (%.1f MB)", kind.name, name, float64(size)/1e6)
	if err := s.client.UploadParts(ctx, ops, f); err != nil {
		return res.ID, fmt.Errorf("upload %s %s: %w", kind.name, name, err)
	}

	*doing = "committing " + kind.name + " " + name
	if err := kind.commit(ctx, s.client, res.ID, sum); err != nil {
		return res.ID, fmt.Errorf("commit upload of %s %s: %w", kind.name, name, err)
	}

	*doing = "waiting for Apple to process " + kind.name + " " + name
	return res.ID, s.waitForAsset(ctx, kind, res.ID, name, start, timeout)
}

// waitForAsset polls the asset until Apple finished or rejected it. Apple
// keeps assets in UPLOAD_COMPLETE for minutes, so every poll reports on stderr.
func (s *Service) waitForAsset(ctx context.Context, kind assetKind, id, name string, start time.Time, timeout time.Duration) error {
	seen := map[string]bool{}
	for {
		st, err := kind.state(ctx, s.client, id)
		if err != nil {
			return fmt.Errorf("get %s %s: %w", kind.name, id, err)
		}
		for _, w := range st.Warnings {
			if msg := stateDetail(asc.StateDetail(w)); !seen[msg] {
				seen[msg] = true
				s.report.Warnf("%s %s: %s", kind.name, name, msg)
			}
		}
		switch asc.AppMediaAssetStateState(st.State) {
		case asc.AppMediaAssetStateStateComplete:
			return nil
		case asc.AppMediaAssetStateStateFailed:
			msgs := make([]string, len(st.Errors))
			for i, e := range st.Errors {
				msgs[i] = stateDetail(asc.StateDetail(e))
			}
			if len(msgs) == 0 {
				msgs = []string{"no error details given"}
			}
			return clierr.New("Apple rejected %s %s:\n  %s", kind.name, name, strings.Join(msgs, "\n  "))
		}
		state := st.State
		if state == "" {
			state = "no state yet"
		}
		elapsed := s.now().Sub(start)
		if elapsed >= timeout {
			return clierr.New("timed out after %s waiting for Apple to process %s %s (state %s)", timeout, kind.name, name, state)
		}
		s.report.Progressf("waiting for Apple to process %s %s (%s, %s elapsed)", kind.name, name, state, shortDuration(elapsed))
		if err := s.poll.Sleep(ctx, min(s.poll.Interval, timeout-elapsed)); err != nil {
			return err
		}
	}
}

// assetOperations converts the upload operations of an asset reservation; it
// shares the conversion (and its checks) with the build upload files.
func assetOperations(in []asc.UploadOperation) ([]ascclient.UploadOperation, error) {
	conv := make([]asc.DeliveryFileUploadOperation, len(in))
	for i, op := range in {
		conv[i] = asc.DeliveryFileUploadOperation{Method: op.Method, URL: op.URL, RequestHeaders: op.RequestHeaders}
		if op.Offset != nil {
			conv[i].Offset = new(int64(*op.Offset))
		}
		if op.Length != nil {
			conv[i].Length = new(int64(*op.Length))
		}
	}
	return uploadOperations(conv)
}
