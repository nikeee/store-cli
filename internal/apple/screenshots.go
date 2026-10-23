package apple

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/nikeee/store-cli/internal/apple/asc"
	"github.com/nikeee/store-cli/internal/apple/ascclient"
	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/media"
	"github.com/nikeee/store-cli/internal/metadata"
	"github.com/nikeee/store-cli/internal/output"
)

// maxScreenshots is the most screenshots App Store Connect keeps in one set.
const maxScreenshots = 10

// screenshotsScope is the metadata.Change scope of screenshot changes.
const screenshotsScope = "screenshots"

// Screenshot is one screenshot of a set. State is assetDeliveryState (e.g.
// COMPLETE, or UPLOAD_COMPLETE while Apple still processes it).
type Screenshot struct {
	ID       string `json:"id"`
	FileName string `json:"fileName"`
	Position int    `json:"position"`
	Checksum string `json:"checksum"`
	State    string `json:"state"`
}

// ScreenshotSet is the screenshots of one locale and device. Device is the
// CLI alias, or the display type for one the CLI has no alias for.
type ScreenshotSet struct {
	Locale      string       `json:"locale"`
	Device      string       `json:"device"`
	DisplayType string       `json:"displayType"`
	Screenshots []Screenshot `json:"screenshots"`
}

// ScreenshotSets is the result of ListScreenshots. Its text form names locale,
// device and screenshot count of every set.
type ScreenshotSets []ScreenshotSet

// RenderText implements output.TextRenderer.
func (l ScreenshotSets) RenderText(w io.Writer, s output.Styles) error {
	data := make([]mediaSetData, len(l))
	for i, set := range l {
		data[i] = mediaSetData{locale: set.Locale, device: set.Device, setType: set.DisplayType, items: set.Screenshots}
	}
	return renderMediaSets(w, s, screenshotMedia, data)
}

// ScreenshotsDeleted is the result of deleting a screenshot set. Action is
// delete, or skip when there was no set.
type ScreenshotsDeleted struct {
	Locale string          `json:"locale"`
	Device string          `json:"device"`
	Action metadata.Action `json:"action"`
}

// VersionLocalizationIDs returns the appStoreVersionLocalizations ID of v per
// locale.
func (s *Service) VersionLocalizationIDs(ctx context.Context, v Version) (map[string]string, error) {
	return s.versionLocalizationIDs(ctx, v.ID)
}

// DeviceForScreenshots returns the device named by alias, else the one the
// first file's pixel size says (on platform, a --platform value).
func DeviceForScreenshots(platform, alias string, paths []string) (media.Device, error) {
	if alias != "" {
		d, ok := media.DeviceByAlias(alias)
		if !ok {
			return media.Device{}, clierr.Usagef("unknown device %q (use one of: %s)", alias, deviceAliases())
		}
		return d, nil
	}
	if len(paths) == 0 {
		return media.Device{}, clierr.Usagef("no screenshot files given")
	}
	info, err := media.Inspect(paths[0])
	if err != nil {
		return media.Device{}, err
	}
	d, err := media.DetectDevice(platform, info.Width, info.Height)
	if err != nil {
		return media.Device{}, fmt.Errorf("%s: %w", paths[0], err)
	}
	return d, nil
}

func deviceAliases() string {
	aliases := make([]string, len(media.Devices))
	for i, d := range media.Devices {
		aliases[i] = d.Alias
	}
	return strings.Join(aliases, ", ")
}

// CheckScreenshotFiles validates the files against device d (format, alpha
// channel, pixel size, at most 10). It reads only local files.
func CheckScreenshotFiles(d media.Device, paths []string) error {
	if len(paths) == 0 {
		return clierr.Usagef("no screenshot files given")
	}
	if len(paths) > maxScreenshots {
		return clierr.New("%d screenshots given, a set holds at most %d", len(paths), maxScreenshots)
	}
	var c clierr.Collector
	for _, p := range paths {
		c.Add(media.ValidateAppleScreenshot(p, d))
	}
	return c.Err()
}

// ListScreenshots returns the screenshot sets of v, per locale (sorted) and
// device (in device table order). locale "" means all locales, device nil all
// devices. A locale the version does not have is an error.
func (s *Service) ListScreenshots(ctx context.Context, v Version, locale string, device *media.Device) (ScreenshotSets, error) {
	data, err := s.listMedia(ctx, screenshotMedia, v, locale, device)
	if err != nil {
		return nil, err
	}
	out := ScreenshotSets{}
	for _, d := range data {
		out = append(out, ScreenshotSet{Locale: d.locale, Device: d.device, DisplayType: d.setType, Screenshots: d.items})
	}
	return out, nil
}

// ScreenshotUpload is the input of UploadScreenshots.
type ScreenshotUpload struct {
	Locale  string
	Device  media.Device
	Paths   []string
	Replace bool
	// Timeout is the time one file may take until Apple processed it.
	Timeout time.Duration
	DryRun  bool
}

// UploadScreenshots uploads the files to the set of o.Device in o.Locale of v
// and returns the planned or applied change and the resulting set (not read
// with DryRun). The files are validated before the first API call.
func (s *Service) UploadScreenshots(ctx context.Context, v Version, o ScreenshotUpload) (ScreenshotSet, metadata.Change, error) {
	set := ScreenshotSet{Locale: o.Locale, Device: o.Device.Alias, DisplayType: o.Device.DisplayType, Screenshots: []Screenshot{}}
	if err := CheckScreenshotFiles(o.Device, o.Paths); err != nil {
		return set, metadata.Change{}, err
	}
	items, change, err := s.uploadMedia(ctx, screenshotMedia, v, mediaUpload{
		locale: o.Locale, device: o.Device, paths: o.Paths, replace: o.Replace,
		timeout: o.Timeout, dryRun: o.DryRun, asset: screenshotKind,
	})
	set.Screenshots = items
	return set, change, err
}

// SyncScreenshots makes the set of device d in the localization hold exactly
// the files, in order: skip when the ordered MD5 list is equal, replace when
// the set has other screenshots, create when it is missing or empty. With
// dryRun it only reads. Files are validated before the first API call.
func (s *Service) SyncScreenshots(ctx context.Context, localizationID, locale string, d media.Device, paths []string, dryRun bool) (metadata.Change, error) {
	if err := CheckScreenshotFiles(d, paths); err != nil {
		return metadata.Change{}, err
	}
	change, _, err := s.syncMedia(ctx, screenshotMedia, mediaSync{
		localizationID: localizationID,
		mediaUpload: mediaUpload{
			locale: locale, device: d, paths: paths, replace: true,
			dryRun: dryRun, timeout: defaultAssetTimeout, asset: screenshotKind,
		},
	})
	return change, err
}

// DeleteScreenshots deletes the whole set of device d in locale of v. A
// missing locale or set is a skip. With dryRun it only reads and returns the
// planned change.
func (s *Service) DeleteScreenshots(ctx context.Context, v Version, locale string, d media.Device, dryRun bool) (ScreenshotsDeleted, metadata.Change, error) {
	change, err := s.deleteMedia(ctx, screenshotMedia, v, locale, d, dryRun)
	return ScreenshotsDeleted{Locale: locale, Device: d.Alias, Action: change.Action}, change, err
}

// screenshotMedia is the mediaKind of screenshot sets.
var screenshotMedia = mediaKind{
	name:   "screenshot",
	plural: "screenshots",
	scope:  screenshotsScope,
	max:    maxScreenshots,
	typeOf: func(d media.Device) string { return d.DisplayType },
	listSets: func(ctx context.Context, c *ascclient.Client, localizationID, displayType string) ([]remoteSet, error) {
		params := asc.AppStoreVersionLocalizationsAppScreenshotSetsGetToManyRelatedParams{ID: localizationID, Limit: 50}
		if displayType != "" {
			params.FilterScreenshotDisplayType = []string{displayType}
		}
		first, err := c.AppStoreVersionLocalizationsAppScreenshotSetsGetToManyRelated(ctx, params)
		if err != nil {
			return nil, err
		}
		var out []remoteSet
		err = ascclient.Paginate(ctx, c, first, func(p *asc.AppScreenshotSetsResponse) *string {
			for _, d := range p.Data {
				rs := remoteSet{id: d.ID}
				if d.Attributes != nil {
					rs.setType = string(deref(d.Attributes.ScreenshotDisplayType))
				}
				// The filter is applied by Apple; checking again keeps a set of
				// another type from ever being replaced.
				if displayType == "" || rs.setType == displayType {
					out = append(out, rs)
				}
			}
			return p.Links.Next
		})
		return out, err
	},
	listItems: func(ctx context.Context, c *ascclient.Client, setID string) ([]Screenshot, error) {
		first, err := c.AppScreenshotSetsAppScreenshotsGetToManyRelated(ctx, asc.AppScreenshotSetsAppScreenshotsGetToManyRelatedParams{ID: setID, Limit: 50})
		if err != nil {
			return nil, err
		}
		out := []Screenshot{}
		err = ascclient.Paginate(ctx, c, first, func(p *asc.AppScreenshotsResponse) *string {
			for _, d := range p.Data {
				shot := Screenshot{ID: d.ID, Position: len(out) + 1}
				if a := d.Attributes; a != nil {
					shot.FileName = deref(a.FileName)
					shot.Checksum = deref(a.SourceFileChecksum)
					if a.AssetDeliveryState != nil {
						shot.State = string(deref(a.AssetDeliveryState.State))
					}
				}
				out = append(out, shot)
			}
			return p.Links.Next
		})
		return out, err
	},
	createSet: func(ctx context.Context, c *ascclient.Client, localizationID string, d media.Device) (string, error) {
		resp, err := c.AppScreenshotSetsCreateInstance(ctx, asc.AppScreenshotSetCreateRequest{
			Data: asc.AppScreenshotSetCreateRequestData{
				Type:       asc.ResourceTypeAppScreenshotSets,
				Attributes: asc.AppScreenshotSetCreateRequestDataAttributes{ScreenshotDisplayType: asc.ScreenshotDisplayType(d.DisplayType)},
				Relationships: &asc.AppScreenshotSetCreateRequestDataRelationships{
					AppStoreVersionLocalization: &asc.AppScreenshotSetCreateRequestDataRelationshipsAppStoreVersionLocalization{
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
		return c.AppScreenshotSetsDeleteInstance(ctx, asc.AppScreenshotSetsDeleteInstanceParams{ID: id})
	},
	deleteItem: func(ctx context.Context, c *ascclient.Client, id string) error {
		return c.AppScreenshotsDeleteInstance(ctx, asc.AppScreenshotsDeleteInstanceParams{ID: id})
	},
}

// screenshotKind is the assetKind of screenshots.
var screenshotKind = assetKind{
	name: "screenshot",
	reserve: func(ctx context.Context, c *ascclient.Client, setID, fileName string, size int64) (assetReservation, error) {
		resp, err := c.AppScreenshotsCreateInstance(ctx, asc.AppScreenshotCreateRequest{
			Data: asc.AppScreenshotCreateRequestData{
				Type:       asc.ResourceTypeAppScreenshots,
				Attributes: asc.AppScreenshotCreateRequestDataAttributes{FileName: fileName, FileSize: int(size)},
				Relationships: asc.AppScreenshotCreateRequestDataRelationships{
					AppScreenshotSet: asc.AppScreenshotCreateRequestDataRelationshipsAppScreenshotSet{
						Data: asc.ResourceIdentifier{Type: asc.ResourceTypeAppScreenshotSets, ID: setID},
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
		_, err := c.AppScreenshotsUpdateInstance(ctx, asc.AppScreenshotsUpdateInstanceParams{ID: id}, asc.AppScreenshotUpdateRequest{
			Data: asc.AppScreenshotUpdateRequestData{
				Type: asc.ResourceTypeAppScreenshots,
				ID:   id,
				Attributes: &asc.AppScreenshotUpdateRequestDataAttributes{
					Uploaded:           asc.Value(true),
					SourceFileChecksum: asc.Value(md5),
				},
			},
		})
		return err
	},
	state: func(ctx context.Context, c *ascclient.Client, id string) (assetState, error) {
		resp, err := c.AppScreenshotsGetInstance(ctx, asc.AppScreenshotsGetInstanceParams{ID: id})
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
