package apple

import (
	"context"
	"io"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/nikeee/store-cli/internal/apple/asc"
	"github.com/nikeee/store-cli/internal/apple/ascclient"
	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/media"
	"github.com/nikeee/store-cli/internal/metadata"
	"github.com/nikeee/store-cli/internal/output"
)

// maxPreviews is the most app previews App Store Connect keeps in one set.
const maxPreviews = 3

// previewsScope is the metadata.Change scope of preview changes.
const previewsScope = "previews"

// Preview is one app preview of a set; it has the fields of a Screenshot.
type Preview = Screenshot

// PreviewSet is the app previews of one locale and device. Device is the CLI
// alias, or the preview type for one the CLI has no alias for.
type PreviewSet struct {
	Locale      string    `json:"locale"`
	Device      string    `json:"device"`
	PreviewType string    `json:"previewType"`
	Previews    []Preview `json:"previews"`
}

// PreviewSets is the result of ListPreviews. Its text form names locale,
// device and preview count of every set.
type PreviewSets []PreviewSet

// RenderText implements output.TextRenderer.
func (l PreviewSets) RenderText(w io.Writer, s output.Styles) error {
	data := make([]mediaSetData, len(l))
	for i, set := range l {
		data[i] = mediaSetData{locale: set.Locale, device: set.Device, setType: set.PreviewType, items: set.Previews}
	}
	return renderMediaSets(w, s, previewMedia, data)
}

// PreviewsDeleted is the result of deleting a preview set. Action is delete,
// or skip when there was no set.
type PreviewsDeleted struct {
	Locale string          `json:"locale"`
	Device string          `json:"device"`
	Action metadata.Action `json:"action"`
}

// DeviceForPreviews returns the device named by alias; a device without app
// previews (the watches) is a usage error.
func DeviceForPreviews(alias string) (media.Device, error) {
	d, err := DeviceForScreenshots("", alias, nil)
	if err != nil {
		return media.Device{}, err
	}
	if d.PreviewType == "" {
		return media.Device{}, clierr.Usagef("device %s has no app previews", d.Alias)
	}
	return d, nil
}

// previewMimeTypes maps the accepted video extensions to their MIME type.
var previewMimeTypes = map[string]string{".mp4": "video/mp4", ".mov": "video/quicktime", ".m4v": "video/x-m4v"}

func previewMimeType(path string) (string, error) {
	if m, ok := previewMimeTypes[strings.ToLower(filepath.Ext(path))]; ok {
		return m, nil
	}
	return "", clierr.New("%s: unsupported video file (use .mp4, .mov or .m4v)", path)
}

// posterFrameRe is a timecode hours:minutes:seconds:frames, like 00:00:05:00.
var posterFrameRe = regexp.MustCompile(`^\d{2}:[0-5]\d:[0-5]\d:\d{2}$`)

// CheckPosterFrame checks the --poster-frame timecode; empty is fine.
func CheckPosterFrame(tc string) error {
	if tc != "" && !posterFrameRe.MatchString(tc) {
		return clierr.Usagef("invalid --poster-frame %q (use hours:minutes:seconds:frames, e.g. 00:00:05:00)", tc)
	}
	return nil
}

// CheckPreviewFiles validates the files (extension, at most 3). It reads no
// file content.
func CheckPreviewFiles(paths []string) error {
	if len(paths) == 0 {
		return clierr.Usagef("no preview files given")
	}
	if len(paths) > maxPreviews {
		return clierr.New("%d previews given, a set holds at most %d", len(paths), maxPreviews)
	}
	for _, p := range paths {
		if _, err := previewMimeType(p); err != nil {
			return err
		}
	}
	return nil
}

// ListPreviews returns the preview sets of v, per locale (sorted) and device
// (in device table order). locale "" means all locales, device nil all
// devices. A locale the version does not have is an error.
func (s *Service) ListPreviews(ctx context.Context, v Version, locale string, device *media.Device) (PreviewSets, error) {
	data, err := s.listMedia(ctx, previewMedia, v, locale, device)
	if err != nil {
		return nil, err
	}
	out := PreviewSets{}
	for _, d := range data {
		out = append(out, PreviewSet{Locale: d.locale, Device: d.device, PreviewType: d.setType, Previews: d.items})
	}
	return out, nil
}

// PreviewUpload is the input of UploadPreviews.
type PreviewUpload struct {
	Locale  string
	Device  media.Device
	Paths   []string
	Replace bool
	// PosterFrame is the timecode of the poster frame of every file; empty
	// leaves the choice to Apple.
	PosterFrame string
	// Timeout is the time one file may take until Apple processed it.
	Timeout time.Duration
	DryRun  bool
}

// UploadPreviews uploads the files to the set of o.Device in o.Locale of v and
// returns the resulting set (not read with DryRun) and the planned or applied
// change. The files are validated before the first API call.
func (s *Service) UploadPreviews(ctx context.Context, v Version, o PreviewUpload) (PreviewSet, metadata.Change, error) {
	set := PreviewSet{Locale: o.Locale, Device: o.Device.Alias, PreviewType: o.Device.PreviewType, Previews: []Preview{}}
	if err := CheckPosterFrame(o.PosterFrame); err != nil {
		return set, metadata.Change{}, err
	}
	if err := CheckPreviewFiles(o.Paths); err != nil {
		return set, metadata.Change{}, err
	}
	items, change, err := s.uploadMedia(ctx, previewMedia, v, mediaUpload{
		locale: o.Locale, device: o.Device, paths: o.Paths, replace: o.Replace, posterFrame: o.PosterFrame,
		timeout: o.Timeout, dryRun: o.DryRun, asset: previewKind(o.PosterFrame),
	})
	set.Previews = items
	return set, change, err
}

// DeletePreviews deletes the whole set of device d in locale of v. A missing
// locale or set is a skip. With dryRun it only reads and returns the planned
// change.
func (s *Service) DeletePreviews(ctx context.Context, v Version, locale string, d media.Device, dryRun bool) (PreviewsDeleted, metadata.Change, error) {
	change, err := s.deleteMedia(ctx, previewMedia, v, locale, d, dryRun)
	return PreviewsDeleted{Locale: locale, Device: d.Alias, Action: change.Action}, change, err
}

// previewMedia is the mediaKind of app preview sets.
var previewMedia = mediaKind{
	name:   "preview",
	plural: "previews",
	scope:  previewsScope,
	max:    maxPreviews,
	typeOf: func(d media.Device) string { return d.PreviewType },
	listSets: func(ctx context.Context, c *ascclient.Client, localizationID, previewType string) ([]remoteSet, error) {
		params := asc.AppStoreVersionLocalizationsAppPreviewSetsGetToManyRelatedParams{ID: localizationID, Limit: 50}
		if previewType != "" {
			params.FilterPreviewType = []string{previewType}
		}
		first, err := c.AppStoreVersionLocalizationsAppPreviewSetsGetToManyRelated(ctx, params)
		if err != nil {
			return nil, err
		}
		var out []remoteSet
		err = ascclient.Paginate(ctx, c, first, func(p *asc.AppPreviewSetsResponse) *string {
			for _, d := range p.Data {
				rs := remoteSet{id: d.ID}
				if d.Attributes != nil {
					rs.setType = string(deref(d.Attributes.PreviewType))
				}
				// The filter is applied by Apple; checking again keeps a set of
				// another type from ever being replaced.
				if previewType == "" || rs.setType == previewType {
					out = append(out, rs)
				}
			}
			return p.Links.Next
		})
		return out, err
	},
	listItems: func(ctx context.Context, c *ascclient.Client, setID string) ([]Preview, error) {
		first, err := c.AppPreviewSetsAppPreviewsGetToManyRelated(ctx, asc.AppPreviewSetsAppPreviewsGetToManyRelatedParams{ID: setID, Limit: 50})
		if err != nil {
			return nil, err
		}
		out := []Preview{}
		err = ascclient.Paginate(ctx, c, first, func(p *asc.AppPreviewsResponse) *string {
			for _, d := range p.Data {
				pv := Preview{ID: d.ID, Position: len(out) + 1}
				if a := d.Attributes; a != nil {
					pv.FileName = deref(a.FileName)
					pv.Checksum = deref(a.SourceFileChecksum)
					pv.PosterFrame = deref(a.PreviewFrameTimeCode)
					if a.AssetDeliveryState != nil {
						pv.State = string(deref(a.AssetDeliveryState.State))
					}
				}
				out = append(out, pv)
			}
			return p.Links.Next
		})
		return out, err
	},
	createSet: func(ctx context.Context, c *ascclient.Client, localizationID string, d media.Device) (string, error) {
		resp, err := c.AppPreviewSetsCreateInstance(ctx, asc.AppPreviewSetCreateRequest{
			Data: asc.AppPreviewSetCreateRequestData{
				Type:       asc.ResourceTypeAppPreviewSets,
				Attributes: asc.AppPreviewSetCreateRequestDataAttributes{PreviewType: asc.PreviewType(d.PreviewType)},
				Relationships: &asc.AppPreviewSetCreateRequestDataRelationships{
					AppStoreVersionLocalization: &asc.AppPreviewSetCreateRequestDataRelationshipsAppStoreVersionLocalization{
						Data: &asc.ResourceIdentifier{Type: asc.ResourceTypeAppStoreVersionLocalizations, ID: localizationID},
					},
				},
			},
		})
		if err != nil {
			return "", err
		}
		return resp.Data.ID, nil
	},
	deleteSet: func(ctx context.Context, c *ascclient.Client, id string) error {
		return c.AppPreviewSetsDeleteInstance(ctx, asc.AppPreviewSetsDeleteInstanceParams{ID: id})
	},
	deleteItem: func(ctx context.Context, c *ascclient.Client, id string) error {
		return c.AppPreviewsDeleteInstance(ctx, asc.AppPreviewsDeleteInstanceParams{ID: id})
	},
}

// previewKind is the assetKind of app previews. posterFrame, if not empty, is
// sent with the commit as previewFrameTimeCode.
func previewKind(posterFrame string) assetKind {
	return assetKind{
		name: "preview",
		reserve: func(ctx context.Context, c *ascclient.Client, setID, fileName string, size int64) (assetReservation, error) {
			mime, err := previewMimeType(fileName)
			if err != nil {
				return assetReservation{}, err
			}
			resp, err := c.AppPreviewsCreateInstance(ctx, asc.AppPreviewCreateRequest{
				Data: asc.AppPreviewCreateRequestData{
					Type:       asc.ResourceTypeAppPreviews,
					Attributes: asc.AppPreviewCreateRequestDataAttributes{FileName: fileName, FileSize: int(size), MimeType: asc.Value(mime)},
					Relationships: asc.AppPreviewCreateRequestDataRelationships{
						AppPreviewSet: asc.AppPreviewCreateRequestDataRelationshipsAppPreviewSet{
							Data: asc.ResourceIdentifier{Type: asc.ResourceTypeAppPreviewSets, ID: setID},
						},
					},
				},
			})
			if err != nil {
				return assetReservation{}, err
			}
			res := assetReservation{ID: resp.Data.ID}
			if a := resp.Data.Attributes; a != nil {
				res.Ops = a.UploadOperations
			}
			return res, nil
		},
		commit: func(ctx context.Context, c *ascclient.Client, id, md5 string) error {
			attrs := &asc.AppPreviewUpdateRequestDataAttributes{Uploaded: asc.Value(true), SourceFileChecksum: asc.Value(md5)}
			if posterFrame != "" {
				attrs.PreviewFrameTimeCode = asc.Value(posterFrame)
			}
			_, err := c.AppPreviewsUpdateInstance(ctx, asc.AppPreviewsUpdateInstanceParams{ID: id}, asc.AppPreviewUpdateRequest{
				Data: asc.AppPreviewUpdateRequestData{Type: asc.ResourceTypeAppPreviews, ID: id, Attributes: attrs},
			})
			return err
		},
		state: func(ctx context.Context, c *ascclient.Client, id string) (assetState, error) {
			resp, err := c.AppPreviewsGetInstance(ctx, asc.AppPreviewsGetInstanceParams{ID: id})
			if err != nil {
				return assetState{}, err
			}
			var st assetState
			if a := resp.Data.Attributes; a != nil && a.AssetDeliveryState != nil {
				d := a.AssetDeliveryState
				st = assetState{State: string(deref(d.State)), Errors: d.Errors, Warnings: d.Warnings}
			}
			return st, nil
		},
	}
}
