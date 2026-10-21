package google

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"text/tabwriter"

	"google.golang.org/api/googleapi"

	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/media"
	"github.com/nikeee/store-cli/internal/metadata"
	"github.com/nikeee/store-cli/internal/output"
)

// ActionDelete is the action of the change planned by DeleteImages.
const ActionDelete metadata.Action = "delete"

// scopeImages is the metadata.Change scope of image changes.
const scopeImages = "images"

// phoneMinImages is the fewest phone screenshots Play accepts (docs/commands.md).
const phoneMinImages = 2

// Image is one image of a listing as Play stores it.
type Image struct {
	ID     string `json:"id"`
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}

// ImageSet is the images of one image type in one language, in Play's order.
// Locale is the Play language code, Type the CLI alias of the image type.
type ImageSet struct {
	Locale string  `json:"locale"`
	Type   string  `json:"type"`
	Images []Image `json:"images"`
}

// ImageSets is the result of ListImages.
type ImageSets []ImageSet

// ImagesDeleted is the result of DeleteImages.
type ImagesDeleted struct {
	Locale  string `json:"locale"`
	Type    string `json:"type"`
	Deleted int    `json:"deleted"`
}

// RenderText implements output.TextRenderer.
func (set ImageSet) RenderText(w io.Writer, s output.Styles) error {
	return ImageSets{set}.RenderText(w, s)
}

// RenderText implements output.TextRenderer.
func (l ImageSets) RenderText(w io.Writer, s output.Styles) error {
	if len(l) == 0 {
		_, err := io.WriteString(w, s.Dim.Render("(none)")+"\n")
		return err
	}
	for i, set := range l {
		if i > 0 {
			if _, err := io.WriteString(w, "\n"); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(w, s.Key.Render(fmt.Sprintf("%s %s: %s", set.Locale, set.Type, imageCount(len(set.Images))))+"\n"); err != nil {
			return err
		}
		if len(set.Images) == 0 {
			continue
		}
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(tw, "  POS\tID\tSHA256\tURL")
		for j, img := range set.Images {
			_, _ = fmt.Fprintf(tw, "  %d\t%s\t%s\t%s\n", j+1, img.ID, img.SHA256, img.URL)
		}
		if err := tw.Flush(); err != nil {
			return err
		}
	}
	return nil
}

func imageCount(n int) string {
	if n == 1 {
		return "1 image"
	}
	return fmt.Sprintf("%d images", n)
}

// CheckImageFiles validates the files of an image type locally: their number
// and each file's format, size and aspect ratio. replace tells whether they
// become the whole set (then a phone set needs its minimum of two).
func CheckImageFiles(t media.ImageType, paths []string, replace bool) error {
	switch {
	case len(paths) == 0:
		return clierr.New("no image files given")
	case !t.Multiple && len(paths) != 1:
		return clierr.New("%s takes exactly one image, got %d", t.Alias, len(paths))
	case len(paths) > t.Max:
		return clierr.New("%s takes at most %d images, got %d", t.Alias, t.Max, len(paths))
	case replace && t.Alias == "phone" && len(paths) < phoneMinImages:
		return clierr.New("phone needs at least %d screenshots, got %d", phoneMinImages, len(paths))
	}
	for _, p := range paths {
		if err := media.ValidateGoogleImage(p, t); err != nil {
			return err
		}
	}
	return nil
}

// listImages returns the images of language and type in the edit.
func (s *Service) listImages(ctx context.Context, editID, language string, t media.ImageType) ([]Image, error) {
	r, err := s.api.Edits.Images.List(s.packageName, editID, language, t.APIType).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("list %s images of %s: %w", t.Alias, language, err)
	}
	out := make([]Image, len(r.Images))
	for i, a := range r.Images {
		out[i] = Image{ID: a.Id, URL: a.Url, SHA256: a.Sha256}
	}
	return out, nil
}

// ListImages returns the images of language for image type t; with t == nil
// every type that holds images, in the order of media.ImageTypes.
func (s *Service) ListImages(ctx context.Context, language string, t *media.ImageType) (ImageSets, error) {
	types := media.ImageTypes
	if t != nil {
		types = []media.ImageType{*t}
	}
	out := ImageSets{}
	err := s.withEdit(ctx, readOnly, func(editID string) error {
		for _, ty := range types {
			imgs, err := s.listImages(ctx, editID, language, ty)
			if err != nil {
				return err
			}
			if t != nil || len(imgs) > 0 {
				out = append(out, ImageSet{Locale: language, Type: ty.Alias, Images: imgs})
			}
		}
		return nil
	})
	return out, err
}

// SyncImages makes the images of language and type t exactly paths, in order:
// nothing if Play's ordered SHA-256 list already equals the files', else all
// images are deleted and the files uploaded. The files are validated before
// the first request. With dryRun it only reads. An error after the set was
// changed says it is incomplete; withEdit then discards the edit.
func (s *Service) SyncImages(ctx context.Context, editID, language string, t media.ImageType, paths []string, dryRun bool) (metadata.Change, error) {
	return s.syncImages(ctx, editID, language, t, paths, true, dryRun)
}

// syncImages is SyncImages, or with replace false the appending upload. A
// single-image type always replaces its one image.
func (s *Service) syncImages(ctx context.Context, editID, language string, t media.ImageType, paths []string, replace, dryRun bool) (metadata.Change, error) {
	replace = replace || !t.Multiple
	if err := CheckImageFiles(t, paths, replace); err != nil {
		return metadata.Change{}, err
	}
	local, err := media.SHA256List(paths)
	if err != nil {
		return metadata.Change{}, err
	}
	change := metadata.Change{Scope: scopeImages, Locale: language, Field: t.Alias}
	remote, err := s.listImages(ctx, editID, language, t)
	if err != nil {
		return change, err
	}
	change.Old = imageCount(len(remote))
	remoteSums := make([]string, len(remote))
	for i, r := range remote {
		remoteSums[i] = r.SHA256
	}
	switch {
	case replace && slices.EqualFunc(remoteSums, local, strings.EqualFold):
		change.Action, change.New = metadata.ActionSkip, change.Old
		return change, nil
	case replace:
		change.New = imageCount(len(paths))
	default:
		total := len(remote) + len(paths)
		if total > t.Max {
			return change, clierr.New("%s %s already holds %d images, adding %d would exceed the maximum of %d (use --replace)",
				language, t.Alias, len(remote), len(paths), t.Max)
		}
		if t.Alias == "phone" && total < phoneMinImages {
			return change, clierr.New("%s phone would hold %d screenshot, Play needs at least %d", language, total, phoneMinImages)
		}
		change.New = imageCount(total)
	}
	switch {
	case len(remote) == 0:
		change.Action = metadata.ActionCreate
	case replace:
		change.Action = metadata.ActionReplace
	default:
		change.Action = metadata.ActionUpdate
	}
	if dryRun {
		return change, nil
	}

	changed := false
	fail := func(err error) error {
		if changed {
			return fmt.Errorf("%s %s: %w (image set incomplete, run again; the edit was discarded, so nothing was published)", language, t.Alias, err)
		}
		return err
	}
	if replace && len(remote) > 0 {
		if _, err := s.api.Edits.Images.Deleteall(s.packageName, editID, language, t.APIType).Context(ctx).Do(); err != nil {
			return change, fail(fmt.Errorf("delete %s images of %s: %w", t.Alias, language, err))
		}
		changed = true
	}
	for i, p := range paths {
		s.report.Progressf("%s %s: image %d of %d", language, t.Alias, i+1, len(paths))
		if err := s.uploadImage(ctx, editID, language, t, p); err != nil {
			return change, fail(err)
		}
		changed = true
	}
	return change, nil
}

// uploadImage streams the file to Play.
func (s *Service) uploadImage(ctx context.Context, editID, language string, t media.ImageType, path string) error {
	info, err := media.Inspect(path)
	if err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return clierr.New("open %s: %v", path, err)
	}
	defer func() { _ = f.Close() }()
	_, err = s.api.Edits.Images.Upload(s.packageName, editID, language, t.APIType).Context(ctx).
		Media(f, googleapi.ContentType("image/"+info.Format)).Do()
	if err != nil {
		return fmt.Errorf("upload %s: %w", path, err)
	}
	return nil
}

// UploadImages uploads the files as images of language and type t in one
// edit. With replace the set becomes exactly these files (see SyncImages),
// else they are appended; single-image types always replace. The files are
// validated before the first request. With DryRun it only reads, and the set
// is the current one. The change is also returned when nothing is changed.
func (s *Service) UploadImages(ctx context.Context, language string, t media.ImageType, paths []string, replace bool, c CommitOptions) (ImageSet, metadata.Change, error) {
	if err := CheckImageFiles(t, paths, replace || !t.Multiple); err != nil {
		return ImageSet{}, metadata.Change{}, err
	}
	var (
		set    ImageSet
		change metadata.Change
	)
	err := s.withEdit(ctx, c, func(editID string) error {
		var err error
		if change, err = s.syncImages(ctx, editID, language, t, paths, replace, c.DryRun); err != nil {
			return err
		}
		imgs, err := s.listImages(ctx, editID, language, t)
		if err != nil {
			return err
		}
		set = ImageSet{Locale: language, Type: t.Alias, Images: imgs}
		if change.Action == metadata.ActionSkip {
			return errNoChange
		}
		return nil
	})
	if errors.Is(err, errNoChange) {
		err = nil
	}
	return set, change, err
}

// DeleteImages deletes all images of language and type t. A set without
// images is an error. With DryRun it only reads and returns the planned change.
func (s *Service) DeleteImages(ctx context.Context, language string, t media.ImageType, c CommitOptions) (ImagesDeleted, metadata.Change, error) {
	res := ImagesDeleted{Locale: language, Type: t.Alias}
	var change metadata.Change
	err := s.withEdit(ctx, c, func(editID string) error {
		imgs, err := s.listImages(ctx, editID, language, t)
		if err != nil {
			return err
		}
		if len(imgs) == 0 {
			return clierr.New("%s has no %s images", language, t.Alias)
		}
		change = metadata.Change{Action: ActionDelete, Scope: scopeImages, Locale: language, Field: t.Alias, Old: imageCount(len(imgs))}
		if c.DryRun {
			return nil
		}
		r, err := s.api.Edits.Images.Deleteall(s.packageName, editID, language, t.APIType).Context(ctx).Do()
		if err != nil {
			return fmt.Errorf("delete %s images of %s: %w", t.Alias, language, err)
		}
		res.Deleted = len(r.Deleted)
		return nil
	})
	return res, change, err
}
