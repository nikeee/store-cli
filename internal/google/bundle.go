package google

import (
	"context"
	"fmt"
	"os"

	"google.golang.org/api/androidpublisher/v3"
	"google.golang.org/api/googleapi"
)

// bundleChunkSize is the chunk size of resumable uploads; a variable so tests
// can exercise the resumable path with small files.
var bundleChunkSize = 8 << 20

const octetStream = "application/octet-stream"

// BundleUploadOptions are the inputs of UploadBundle.
type BundleUploadOptions struct {
	Path         string // AAB file
	Mapping      string // optional ProGuard/R8 mapping.txt
	Tracks       []string
	ReleaseName  string
	Status       string            // API spelling, see the Status constants
	UserFraction float64           // for inProgress
	Notes        map[string]string // App Store style locale -> text
	Commit       CommitOptions
}

// BundleUploadResult is the result of UploadBundle. Under DryRun VersionCode
// is 0 and the releases have no version codes: nothing was uploaded.
type BundleUploadResult struct {
	VersionCode int64   `json:"versionCode"`
	Tracks      []Track `json:"tracks"`
}

// UploadBundle uploads the AAB (and the mapping file) and puts a release with
// its version code on every track, all in one edit. The files are streamed
// from disk.
func (s *Service) UploadBundle(ctx context.Context, o BundleUploadOptions) (BundleUploadResult, error) {
	var res BundleUploadResult
	err := s.withEdit(ctx, o.Commit, func(editID string) error {
		res = BundleUploadResult{}
		notes, err := s.releaseNotes(ctx, editID, o.Notes)
		if err != nil {
			return err
		}
		var codes googleapi.Int64s
		if !o.Commit.DryRun {
			if res.VersionCode, err = s.uploadAAB(ctx, editID, o.Path); err != nil {
				return err
			}
			codes = googleapi.Int64s{res.VersionCode}
			if o.Mapping != "" {
				if err := s.uploadMapping(ctx, editID, res.VersionCode, o.Mapping); err != nil {
					return err
				}
			}
		}
		for _, t := range o.Tracks {
			rel := &androidpublisher.TrackRelease{
				Name: o.ReleaseName, Status: o.Status, VersionCodes: codes,
				UserFraction: o.UserFraction, ReleaseNotes: notes,
			}
			track, err := s.setRelease(ctx, editID, t, rel, o.Commit.DryRun)
			if err != nil {
				return err
			}
			res.Tracks = append(res.Tracks, track)
		}
		return nil
	})
	return res, err
}

func (s *Service) uploadAAB(ctx context.Context, editID, path string) (int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer func() { _ = f.Close() }()
	s.report.Progressf("uploading %s", path)
	b, err := s.api.Edits.Bundles.Upload(s.packageName, editID).Context(ctx).
		Media(f, googleapi.ContentType(octetStream), googleapi.ChunkSize(bundleChunkSize)).Do()
	if err != nil {
		return 0, fmt.Errorf("upload bundle: %w", err)
	}
	return b.VersionCode, nil
}

func (s *Service) uploadMapping(ctx context.Context, editID string, versionCode int64, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	_, err = s.api.Edits.Deobfuscationfiles.Upload(s.packageName, editID, versionCode, "proguard").Context(ctx).
		Media(f, googleapi.ContentType(octetStream), googleapi.ChunkSize(bundleChunkSize)).Do()
	if err != nil {
		return fmt.Errorf("upload mapping file: %w", err)
	}
	return nil
}
