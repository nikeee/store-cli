package apple

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/nikeee/store-cli/internal/apple/asc"
	"github.com/nikeee/store-cli/internal/apple/ascclient"
	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/media"
	"github.com/nikeee/store-cli/internal/metadata"
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

// ScreenshotsDeleted is the result of deleting a screenshot set.
type ScreenshotsDeleted struct {
	Locale  string `json:"locale"`
	Device  string `json:"device"`
	Deleted bool   `json:"deleted"`
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
	for _, p := range paths {
		if err := media.ValidateAppleScreenshot(p, d); err != nil {
			return err
		}
	}
	return nil
}

// ListScreenshots returns the screenshot sets of v, per locale (sorted) and
// device (in device table order). locale "" means all locales, device nil all
// devices. A locale the version does not have is an error.
func (s *Service) ListScreenshots(ctx context.Context, v Version, locale string, device *media.Device) ([]ScreenshotSet, error) {
	ids, err := s.VersionLocalizationIDs(ctx, v)
	if err != nil {
		return nil, err
	}
	var locales []string
	if locale != "" {
		if _, ok := ids[locale]; !ok {
			return nil, noLocalization(v, locale)
		}
		locales = []string{locale}
	} else {
		for l := range ids {
			locales = append(locales, l)
		}
		slices.Sort(locales)
	}
	displayType := ""
	if device != nil {
		displayType = device.DisplayType
	}
	out := []ScreenshotSet{}
	for _, l := range locales {
		sets, err := s.screenshotSets(ctx, ids[l], displayType)
		if err != nil {
			return nil, fmt.Errorf("list screenshot sets of %s: %w", l, err)
		}
		slices.SortStableFunc(sets, func(a, b remoteSet) int { return deviceRank(a.displayType) - deviceRank(b.displayType) })
		for _, rs := range sets {
			shots, err := s.setScreenshots(ctx, rs.id)
			if err != nil {
				return nil, fmt.Errorf("list screenshots of %s %s: %w", l, rs.displayType, err)
			}
			out = append(out, ScreenshotSet{Locale: l, Device: deviceName(rs.displayType), DisplayType: rs.displayType, Screenshots: shots})
		}
	}
	return out, nil
}

func noLocalization(v Version, locale string) *clierr.Error {
	e := clierr.New("version %s has no localization for %s", v.Version, locale)
	e.Hint = "store apple listing locale add " + locale
	return e
}

// deviceRank is the position of the device with displayType in media.Devices;
// unknown display types sort last.
func deviceRank(displayType string) int {
	if i := slices.IndexFunc(media.Devices, func(d media.Device) bool { return d.DisplayType == displayType }); i >= 0 {
		return i
	}
	return len(media.Devices)
}

// deviceName is the alias of the device with displayType, else displayType.
func deviceName(displayType string) string {
	if i := deviceRank(displayType); i < len(media.Devices) {
		return media.Devices[i].Alias
	}
	return displayType
}

// remoteSet is an appScreenshotSet of a localization.
type remoteSet struct{ id, displayType string }

// screenshotSets returns the sets of the localization, only those of
// displayType if given.
func (s *Service) screenshotSets(ctx context.Context, localizationID, displayType string) ([]remoteSet, error) {
	params := asc.AppStoreVersionLocalizationsAppScreenshotSetsGetToManyRelatedParams{ID: localizationID, Limit: 50}
	if displayType != "" {
		params.FilterScreenshotDisplayType = []string{displayType}
	}
	first, err := s.client.AppStoreVersionLocalizationsAppScreenshotSetsGetToManyRelated(ctx, params)
	if err != nil {
		return nil, err
	}
	var out []remoteSet
	err = ascclient.Paginate(ctx, s.client, first, func(p *asc.AppScreenshotSetsResponse) *string {
		for _, d := range p.Data {
			rs := remoteSet{id: d.ID}
			if d.Attributes != nil {
				rs.displayType = string(deref(d.Attributes.ScreenshotDisplayType))
			}
			// The filter is applied by Apple; checking again keeps a set of
			// another type from ever being replaced.
			if displayType == "" || rs.displayType == displayType {
				out = append(out, rs)
			}
		}
		return p.Links.Next
	})
	return out, err
}

// setScreenshots returns the screenshots of a set in the order Apple shows them.
func (s *Service) setScreenshots(ctx context.Context, setID string) ([]Screenshot, error) {
	first, err := s.client.AppScreenshotSetsAppScreenshotsGetToManyRelated(ctx, asc.AppScreenshotSetsAppScreenshotsGetToManyRelatedParams{ID: setID, Limit: 50})
	if err != nil {
		return nil, err
	}
	out := []Screenshot{}
	err = ascclient.Paginate(ctx, s.client, first, func(p *asc.AppScreenshotsResponse) *string {
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
	if err := CheckScreenshotFiles(o.Device, o.Paths); err != nil {
		return ScreenshotSet{}, metadata.Change{}, err
	}
	ids, err := s.VersionLocalizationIDs(ctx, v)
	if err != nil {
		return ScreenshotSet{}, metadata.Change{}, err
	}
	locID, ok := ids[o.Locale]
	if !ok {
		return ScreenshotSet{}, metadata.Change{}, noLocalization(v, o.Locale)
	}
	change, setID, err := s.syncScreenshots(ctx, screenshotSync{
		localizationID: locID, locale: o.Locale, device: o.Device, paths: o.Paths,
		replace: o.Replace, dryRun: o.DryRun, timeout: o.Timeout,
	})
	set := ScreenshotSet{Locale: o.Locale, Device: o.Device.Alias, DisplayType: o.Device.DisplayType, Screenshots: []Screenshot{}}
	if err != nil || o.DryRun || setID == "" {
		return set, change, err
	}
	if set.Screenshots, err = s.setScreenshots(ctx, setID); err != nil {
		return set, change, fmt.Errorf("list screenshots of %s %s: %w", o.Locale, o.Device.Alias, err)
	}
	return set, change, nil
}

// SyncScreenshots makes the set of device d in the localization hold exactly
// the files, in order: skip when the ordered MD5 list is equal, replace when
// the set has other screenshots, create when it is missing or empty. With
// dryRun it only reads. Files are validated before the first API call.
func (s *Service) SyncScreenshots(ctx context.Context, localizationID, locale string, d media.Device, paths []string, dryRun bool) (metadata.Change, error) {
	if err := CheckScreenshotFiles(d, paths); err != nil {
		return metadata.Change{}, err
	}
	change, _, err := s.syncScreenshots(ctx, screenshotSync{
		localizationID: localizationID, locale: locale, device: d, paths: paths,
		replace: true, dryRun: dryRun, timeout: defaultAssetTimeout,
	})
	return change, err
}

type screenshotSync struct {
	localizationID, locale string
	device                 media.Device
	paths                  []string
	replace, dryRun        bool
	timeout                time.Duration
}

// syncScreenshots returns the change and the ID of the set ("" when a dry run
// would create it). The files are already validated.
func (s *Service) syncScreenshots(ctx context.Context, o screenshotSync) (metadata.Change, string, error) {
	local, err := media.MD5List(o.paths)
	if err != nil {
		return metadata.Change{}, "", err
	}
	change := metadata.Change{Scope: screenshotsScope, Locale: o.locale, Field: o.device.Alias}
	sets, err := s.screenshotSets(ctx, o.localizationID, o.device.DisplayType)
	if err != nil {
		return change, "", fmt.Errorf("list screenshot sets of %s: %w", o.locale, err)
	}
	var setID string
	var remote []Screenshot
	if len(sets) > 0 {
		setID = sets[0].id
		if remote, err = s.setScreenshots(ctx, setID); err != nil {
			return change, setID, fmt.Errorf("list screenshots of %s %s: %w", o.locale, o.device.Alias, err)
		}
	}
	change.Old = countScreenshots(len(remote))

	remoteSums := make([]string, len(remote))
	for i, r := range remote {
		remoteSums[i] = r.Checksum
	}
	switch {
	case o.replace && slices.Equal(remoteSums, local):
		change.Action, change.New = metadata.ActionSkip, change.Old
		return change, setID, nil
	case o.replace:
		change.New = countScreenshots(len(local))
	default:
		if total := len(remote) + len(local); total > maxScreenshots {
			return change, setID, clierr.New("%s %s already holds %d screenshots, adding %d would exceed the maximum of %d (use --replace)",
				o.locale, o.device.Alias, len(remote), len(local), maxScreenshots)
		}
		change.New = countScreenshots(len(remote) + len(local))
	}
	switch {
	case len(remote) == 0:
		change.Action = metadata.ActionCreate
	case o.replace:
		change.Action = metadata.ActionReplace
	default:
		change.Action = metadata.ActionUpdate
	}
	if o.dryRun {
		return change, setID, nil
	}

	if setID == "" {
		if setID, err = s.createScreenshotSet(ctx, o.localizationID, o.device); err != nil {
			return change, "", fmt.Errorf("create %s screenshot set of %s: %w", o.device.Alias, o.locale, err)
		}
	}
	if o.replace {
		for _, r := range remote {
			if err := s.client.AppScreenshotsDeleteInstance(ctx, asc.AppScreenshotsDeleteInstanceParams{ID: r.ID}); err != nil {
				return change, setID, fmt.Errorf("delete screenshot %s: %w", r.FileName, err)
			}
		}
	}
	for i, p := range o.paths {
		s.report.Progressf("%s %s: screenshot %d of %d", o.locale, o.device.Alias, i+1, len(o.paths))
		if _, err := s.uploadAsset(ctx, screenshotKind, setID, p, o.timeout); err != nil {
			return change, setID, err
		}
	}
	return change, setID, nil
}

func countScreenshots(n int) string {
	if n == 1 {
		return "1 screenshot"
	}
	return fmt.Sprintf("%d screenshots", n)
}

func (s *Service) createScreenshotSet(ctx context.Context, localizationID string, d media.Device) (string, error) {
	resp, err := s.client.AppScreenshotSetsCreateInstance(ctx, asc.AppScreenshotSetCreateRequest{
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
}

// DeleteScreenshots deletes the whole set of device d in locale of v. A
// missing locale or set is an error. With dryRun it only reads and returns the
// planned change.
func (s *Service) DeleteScreenshots(ctx context.Context, v Version, locale string, d media.Device, dryRun bool) (ScreenshotsDeleted, metadata.Change, error) {
	res := ScreenshotsDeleted{Locale: locale, Device: d.Alias}
	ids, err := s.VersionLocalizationIDs(ctx, v)
	if err != nil {
		return res, metadata.Change{}, err
	}
	locID, ok := ids[locale]
	if !ok {
		return res, metadata.Change{}, noLocalization(v, locale)
	}
	sets, err := s.screenshotSets(ctx, locID, d.DisplayType)
	if err != nil {
		return res, metadata.Change{}, fmt.Errorf("list screenshot sets of %s: %w", locale, err)
	}
	if len(sets) == 0 {
		return res, metadata.Change{}, clierr.New("%s has no %s screenshot set", locale, d.Alias)
	}
	shots, err := s.setScreenshots(ctx, sets[0].id)
	if err != nil {
		return res, metadata.Change{}, fmt.Errorf("list screenshots of %s %s: %w", locale, d.Alias, err)
	}
	change := metadata.Change{Action: ActionDelete, Scope: screenshotsScope, Locale: locale, Field: d.Alias, Old: countScreenshots(len(shots))}
	if dryRun {
		return res, change, nil
	}
	if err := s.client.AppScreenshotSetsDeleteInstance(ctx, asc.AppScreenshotSetsDeleteInstanceParams{ID: sets[0].id}); err != nil {
		return res, change, fmt.Errorf("delete %s screenshot set of %s: %w", d.Alias, locale, err)
	}
	res.Deleted = true
	return res, change, nil
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
