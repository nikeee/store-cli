package apple

import (
	"context"
	"crypto/md5"
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
	"github.com/nikeee/store-cli/internal/apple/ipa"
	"github.com/nikeee/store-cli/internal/clierr"
)

// UploadOptions are the inputs of UploadBuild. App is the --app value ("" when
// not given) and Platform the --platform value. Timeout bounds the whole
// upload including every wait. Encryption nil leaves export compliance alone.
type UploadOptions struct {
	Path, App, Platform string
	Wait                bool
	Timeout             time.Duration
	Notes, NotesLocale  string
	Encryption          *bool
}

// UploadResult is the result of build upload. BuildID is empty when the
// command did not wait for the build. State is the lowercase state of the
// build upload ("dry-run" for a preview).
type UploadResult struct {
	App           string `json:"app"`
	AppID         string `json:"appId"`
	Version       string `json:"version"`
	Build         string `json:"build"`
	BuildUploadID string `json:"buildUploadId"`
	BuildID       string `json:"buildId"`
	State         string `json:"state"`
}

// uploadPlan is what the IPA and the options say about an upload.
type uploadPlan struct {
	info     ipa.Info
	app      App
	platform asc.Platform
}

// planUpload validates the options, reads the IPA and resolves its app. It
// sends nothing but reads.
func (s *Service) planUpload(ctx context.Context, o UploadOptions) (uploadPlan, error) {
	if err := CheckBuildNotes(o.Notes); err != nil {
		return uploadPlan{}, err
	}
	info, err := ipa.Read(o.Path)
	if err != nil {
		return uploadPlan{}, err
	}
	if o.App != "" && o.App != info.BundleID {
		return uploadPlan{}, clierr.Usagef("IPA has bundle ID %s, --app is %s", info.BundleID, o.App)
	}
	platform, ok := APIPlatform(o.Platform)
	if !ok {
		return uploadPlan{}, clierr.Usagef("invalid --platform %q", o.Platform)
	}
	if string(platform) != info.Platform {
		return uploadPlan{}, clierr.Usagef("IPA is for %s, --platform is %s", info.Platform, o.Platform)
	}
	app, err := s.ResolveApp(ctx, info.BundleID)
	if err != nil {
		return uploadPlan{}, err
	}
	return uploadPlan{info: info, app: app, platform: platform}, nil
}

func (p uploadPlan) result(state string) UploadResult {
	return UploadResult{App: p.app.BundleID, AppID: p.app.ID, Version: p.info.Version, Build: p.info.Build, State: state}
}

// PreviewUpload validates and resolves like UploadBuild but uploads nothing.
func (s *Service) PreviewUpload(ctx context.Context, o UploadOptions) (UploadResult, error) {
	p, err := s.planUpload(ctx, o)
	if err != nil {
		return UploadResult{}, err
	}
	return p.result("dry-run"), nil
}

// UploadBuild uploads the IPA through the buildUploads API and waits until
// Apple accepted it; with Wait, notes or Encryption also until the build
// exists (and with Wait until it is processed).
func (s *Service) UploadBuild(ctx context.Context, o UploadOptions) (UploadResult, error) {
	p, err := s.planUpload(ctx, o)
	if err != nil {
		return UploadResult{}, err
	}
	start := s.now()
	parent := ctx
	ctx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()

	u := &upload{Service: s, plan: p, opts: o, start: start}
	res, err := u.run(ctx)
	if err != nil && parent.Err() == nil && errors.Is(err, context.DeadlineExceeded) {
		err = clierr.New("timed out after %s while %s", o.Timeout, u.doing)
	}
	if err != nil && res.Uploaded() {
		err = u.afterUpload(res, err)
	}
	return res, err
}

// Uploaded reports whether Apple accepted the upload: the build is in App
// Store Connect even when a later step failed.
func (r UploadResult) Uploaded() bool {
	return r.State == strings.ToLower(string(asc.BuildUploadStateComplete))
}

// upload is the state of one UploadBuild call. doing names the current step
// for timeout errors. todo holds, once Apple accepted the upload, the
// commands that do the steps not done yet; it is empty when nothing is left
// to do, e.g. after processing failed.
type upload struct {
	*Service
	plan  uploadPlan
	opts  UploadOptions
	start time.Time
	doing string
	todo  []string
}

// afterUpload reports err of a step after Apple accepted the upload: it says
// that the build was uploaded, and the hint names the commands for the rest.
func (u *upload) afterUpload(res UploadResult, err error) error {
	err = ascclient.MapError(err)
	e := clierr.New("build %s (%s) was uploaded (build upload %s); then: %v", res.Build, res.Version, res.BuildUploadID, err)
	var ce *clierr.Error
	if errors.As(err, &ce) {
		e.Status, e.Code, e.Detail = ce.Status, ce.Code, ce.Detail
	}
	e.Hint = strings.Join(u.todo, " && ")
	return e
}

func (u *upload) run(ctx context.Context) (UploadResult, error) {
	p, o := u.plan, u.opts
	name := filepath.Base(o.Path)
	f, err := os.Open(o.Path)
	if err != nil {
		return UploadResult{}, fmt.Errorf("open %s: %w", o.Path, err)
	}
	defer func() { _ = f.Close() }()
	fi, err := f.Stat()
	if err != nil {
		return UploadResult{}, fmt.Errorf("stat %s: %w", o.Path, err)
	}
	checksum := md5.New()
	if _, err := io.Copy(checksum, f); err != nil {
		return UploadResult{}, fmt.Errorf("read %s: %w", o.Path, err)
	}

	res := p.result("")
	u.doing = "creating the build upload"
	created, err := u.client.BuildUploadsCreateInstance(ctx, asc.BuildUploadCreateRequest{
		Data: asc.BuildUploadCreateRequestData{
			Type: asc.ResourceTypeBuildUploads,
			Attributes: asc.BuildUploadCreateRequestDataAttributes{
				CfBundleShortVersionString: p.info.Version,
				CfBundleVersion:            p.info.Build,
				Platform:                   p.platform,
			},
			Relationships: asc.BuildUploadCreateRequestDataRelationships{
				App: asc.BuildUploadCreateRequestDataRelationshipsApp{Data: asc.ResourceIdentifier{Type: asc.ResourceTypeApps, ID: p.app.ID}},
			},
		},
	})
	if err != nil {
		return res, fmt.Errorf("create build upload: %w", err)
	}
	res.BuildUploadID = created.Data.ID

	file, err := u.client.BuildUploadFilesCreateInstance(ctx, asc.BuildUploadFileCreateRequest{
		Data: asc.BuildUploadFileCreateRequestData{
			Type: asc.ResourceTypeBuildUploadFiles,
			Attributes: asc.BuildUploadFileCreateRequestDataAttributes{
				AssetType: asc.BuildUploadFileCreateRequestDataAttributesAssetTypeAsset,
				FileName:  name,
				FileSize:  fi.Size(),
				Uti:       asc.BuildUploadFileCreateRequestDataAttributesUtiComAppleIpa,
			},
			Relationships: asc.BuildUploadFileCreateRequestDataRelationships{
				BuildUpload: asc.BuildUploadFileCreateRequestDataRelationshipsBuildUpload{Data: asc.ResourceIdentifier{Type: asc.ResourceTypeBuildUploads, ID: created.Data.ID}},
			},
		},
	})
	if err != nil {
		return res, fmt.Errorf("create build upload file: %w", err)
	}
	if file.Data.Attributes == nil {
		return res, clierr.New("App Store Connect returned no upload operations for %s", name)
	}
	ops, err := uploadOperations(file.Data.Attributes.UploadOperations)
	if err != nil {
		return res, err
	}

	u.doing = "uploading " + name
	u.report.Progressf("uploading %s (%.1f MB)", name, float64(fi.Size())/1e6)
	if err := u.client.UploadParts(ctx, ops, f); err != nil {
		return res, fmt.Errorf("upload %s: %w", name, err)
	}

	u.doing = "committing the upload"
	_, err = u.client.BuildUploadFilesUpdateInstance(ctx, asc.BuildUploadFilesUpdateInstanceParams{ID: file.Data.ID}, asc.BuildUploadFileUpdateRequest{
		Data: asc.BuildUploadFileUpdateRequestData{
			Type: asc.ResourceTypeBuildUploadFiles,
			ID:   file.Data.ID,
			Attributes: &asc.BuildUploadFileUpdateRequestDataAttributes{
				Uploaded: asc.Value(true),
				SourceFileChecksums: asc.Value(asc.Checksums{File: &asc.ChecksumsFile{
					Hash:      new(hex.EncodeToString(checksum.Sum(nil))),
					Algorithm: new(asc.ChecksumAlgorithmMd5),
				}}),
			},
		},
	})
	if err != nil {
		return res, fmt.Errorf("commit upload of %s: %w", name, err)
	}
	u.report.Progressf("upload accepted, waiting for Apple")

	state, err := u.waitForUpload(ctx, created.Data.ID)
	res.State = strings.ToLower(state)
	if err != nil {
		return res, err
	}

	build := fmt.Sprintf("--version %s --platform %s", p.info.Version, o.Platform)
	if o.Notes != "" {
		u.todo = append(u.todo, fmt.Sprintf("store apple testflight notes set %s %s --locale %s --text <notes>", p.info.Build, build, o.NotesLocale))
	}
	if o.Encryption != nil {
		u.todo = append(u.todo, fmt.Sprintf("store apple build set-encryption %s %t %s", p.info.Build, *o.Encryption, build))
	}
	if o.Wait {
		u.todo = append(u.todo, fmt.Sprintf("store apple build wait %s %s", p.info.Build, build))
	}
	if len(u.todo) == 0 {
		return res, nil
	}
	b, err := u.waitForBuild(ctx, BuildExists, "to appear")
	if err != nil {
		return res, err
	}
	res.BuildID = b.ID
	if o.Notes != "" {
		u.doing = "setting the notes"
		if _, err := u.SetBuildNotes(ctx, b, o.NotesLocale, o.Notes); err != nil {
			return res, err
		}
		u.todo = u.todo[1:]
		u.report.Progressf("notes set (%s)", o.NotesLocale)
	}
	if o.Encryption != nil {
		u.doing = "setting the export compliance answer"
		if _, err := u.SetEncryption(ctx, b, *o.Encryption); err != nil {
			return res, err
		}
		u.todo = u.todo[1:]
		u.report.Progressf("export compliance set")
	}
	if o.Wait {
		if b, err := u.waitForBuild(ctx, BuildProcessed, "to be processed"); err != nil {
			if b.State == "failed" || b.State == "invalid" {
				u.todo = nil
			}
			return res, err
		}
	}
	return res, nil
}

// waitForBuild waits for the uploaded build with what is left of the timeout.
func (u *upload) waitForBuild(ctx context.Context, until BuildCondition, what string) (Build, error) {
	u.doing = "waiting for build " + u.plan.info.Build + " " + what
	remaining := u.opts.Timeout - u.now().Sub(u.start)
	if remaining <= 0 {
		return Build{}, clierr.New("timed out after %s %s", u.opts.Timeout, u.doing)
	}
	return u.WaitForBuild(ctx, u.plan.app, u.plan.info.Build, u.plan.info.Version, u.opts.Platform, until, remaining)
}

// waitForUpload polls the build upload until Apple accepted or rejected it
// and returns its final state.
func (u *upload) waitForUpload(ctx context.Context, id string) (string, error) {
	u.doing = "waiting for Apple to accept the upload"
	seen := map[string]bool{}
	for {
		resp, err := u.client.BuildUploadsGetInstance(ctx, asc.BuildUploadsGetInstanceParams{ID: id})
		if err != nil {
			return "", fmt.Errorf("get build upload %s: %w", id, err)
		}
		var st asc.BuildUploadAttributesState
		if a := resp.Data.Attributes; a != nil && a.State != nil {
			st = *a.State
		}
		for _, w := range st.Warnings {
			if msg := stateDetail(w); !seen[msg] {
				seen[msg] = true
				u.report.Warnf("%s", msg)
			}
		}
		state := string(deref(st.State))
		switch deref(st.State) {
		case asc.BuildUploadStateComplete:
			return state, nil
		case asc.BuildUploadStateFailed:
			msgs := make([]string, len(st.Errors))
			for i, e := range st.Errors {
				msgs[i] = stateDetail(e)
			}
			if len(msgs) == 0 {
				msgs = []string{"no error details given"}
			}
			e := clierr.New("Apple rejected the upload of build %s:\n  %s", u.plan.info.Build, strings.Join(msgs, "\n  "))
			return state, e
		}
		elapsed := u.now().Sub(u.start)
		if elapsed >= u.opts.Timeout {
			return state, clierr.New("timed out after %s waiting for Apple to accept the upload (state %s)", u.opts.Timeout, state)
		}
		u.report.Progressf("waiting for Apple to accept the upload (%s, %s elapsed)", state, shortDuration(elapsed))
		if err := u.poll.Sleep(ctx, min(u.poll.Interval, u.opts.Timeout-elapsed)); err != nil {
			return state, err
		}
	}
}

func stateDetail(d asc.StateDetail) string {
	return strings.TrimSpace(deref(d.Code) + " " + deref(d.Description))
}

// uploadOperations converts the operations App Store Connect returned.
func uploadOperations(in []asc.DeliveryFileUploadOperation) ([]ascclient.UploadOperation, error) {
	if len(in) == 0 {
		return nil, clierr.New("App Store Connect returned no upload operations")
	}
	ops := make([]ascclient.UploadOperation, len(in))
	for i, op := range in {
		if op.URL == nil || op.Method == nil || op.Offset == nil || op.Length == nil {
			return nil, clierr.New("App Store Connect returned an incomplete upload operation (part %d)", i+1)
		}
		h := map[string]string{}
		for _, hh := range op.RequestHeaders {
			h[deref(hh.Name)] = deref(hh.Value)
		}
		ops[i] = ascclient.UploadOperation{Method: *op.Method, URL: *op.URL, Offset: *op.Offset, Length: *op.Length, Headers: h}
	}
	return ops, nil
}
