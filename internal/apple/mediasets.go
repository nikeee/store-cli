package apple

import (
	"context"
	"fmt"
	"io"
	"slices"
	"text/tabwriter"
	"time"

	"github.com/nikeee/store-cli/internal/apple/asc"
	"github.com/nikeee/store-cli/internal/apple/ascclient"
	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/media"
	"github.com/nikeee/store-cli/internal/metadata"
	"github.com/nikeee/store-cli/internal/output"
)

// mediaKind is what differs between the sets of screenshots and of app
// previews: the API calls, the set type per device and the limits. The list,
// sync and delete logic in this file serves both.
type mediaKind struct {
	// name and plural are used in messages and counts: "screenshot", "screenshots".
	name, plural string
	// scope is the metadata.Change scope.
	scope string
	// max is the most items App Store Connect keeps in one set.
	max int
	// typeOf is the set type of a device (display type or preview type).
	typeOf func(media.Device) string
	// listSets lists the sets of a localization, only those of typ if given.
	listSets func(ctx context.Context, c *ascclient.Client, localizationID, typ string) ([]remoteSet, error)
	// listItems lists the items of a set in the order Apple shows them.
	listItems  func(ctx context.Context, c *ascclient.Client, setID string) ([]Screenshot, error)
	createSet  func(ctx context.Context, c *ascclient.Client, localizationID string, d media.Device) (string, error)
	deleteSet  func(ctx context.Context, c *ascclient.Client, setID string) error
	deleteItem func(ctx context.Context, c *ascclient.Client, id string) error
}

// remoteSet is a set of a localization; setType is its display or preview type.
type remoteSet struct{ id, setType string }

func (k mediaKind) count(n int) string {
	if n == 1 {
		return "1 " + k.name
	}
	return fmt.Sprintf("%d %s", n, k.plural)
}

// rank is the position of the device with set type typ in media.Devices;
// unknown types sort last.
func (k mediaKind) rank(typ string) int {
	if i := slices.IndexFunc(media.Devices, func(d media.Device) bool { return k.typeOf(d) == typ }); i >= 0 {
		return i
	}
	return len(media.Devices)
}

// deviceName is the alias of the device with set type typ, else typ.
func (k mediaKind) deviceName(typ string) string {
	if i := k.rank(typ); i < len(media.Devices) {
		return media.Devices[i].Alias
	}
	return typ
}

func noLocalization(v Version, locale string) *clierr.Error {
	e := clierr.New("version %s has no localization for %s", v.Version, locale)
	e.Hint = "store apple listing locale add " + locale
	return e
}

// mediaSetData is one set with its items, before it is given its public type.
type mediaSetData struct {
	locale, device, setType string
	items                   []Screenshot
}

// listMedia returns the sets of v, per locale (sorted) and device (in device
// table order). locale "" means all locales, device nil all devices. A locale
// the version does not have is an error.
func (s *Service) listMedia(ctx context.Context, k mediaKind, v Version, locale string, device *media.Device) ([]mediaSetData, error) {
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
	typ := ""
	if device != nil {
		typ = k.typeOf(*device)
	}
	var out []mediaSetData
	for _, l := range locales {
		sets, err := k.listSets(ctx, s.client, ids[l], typ)
		if err != nil {
			return nil, fmt.Errorf("list %s sets of %s: %w", k.name, l, err)
		}
		slices.SortStableFunc(sets, func(a, b remoteSet) int { return k.rank(a.setType) - k.rank(b.setType) })
		for _, rs := range sets {
			items, err := k.listItems(ctx, s.client, rs.id)
			if err != nil {
				return nil, fmt.Errorf("list %s of %s %s: %w", k.plural, l, rs.setType, err)
			}
			out = append(out, mediaSetData{locale: l, device: k.deviceName(rs.setType), setType: rs.setType, items: items})
		}
	}
	return out, nil
}

// renderMediaSets writes the text form of sets: a head line per set and a
// table of its items.
func renderMediaSets(w io.Writer, s output.Styles, k mediaKind, sets []mediaSetData) error {
	if len(sets) == 0 {
		_, err := io.WriteString(w, s.Dim.Render("(none)")+"\n")
		return err
	}
	for i, set := range sets {
		if i > 0 {
			if _, err := io.WriteString(w, "\n"); err != nil {
				return err
			}
		}
		head := fmt.Sprintf("%s %s (%s): %s", set.locale, set.device, set.setType, k.count(len(set.items)))
		if _, err := io.WriteString(w, s.Key.Render(head)+"\n"); err != nil {
			return err
		}
		if len(set.items) == 0 {
			continue
		}
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(tw, "  POS\tFILE\tSTATE\tCHECKSUM\tID")
		for _, it := range set.items {
			_, _ = fmt.Fprintf(tw, "  %d\t%s\t%s\t%s\t%s\n", it.Position, it.FileName, it.State, it.Checksum, it.ID)
		}
		if err := tw.Flush(); err != nil {
			return err
		}
	}
	return nil
}

// mediaUpload is the input of uploadMedia. asset is the upload flow of the
// files; the files are already validated.
type mediaUpload struct {
	locale  string
	device  media.Device
	paths   []string
	replace bool
	timeout time.Duration
	dryRun  bool
	asset   assetKind
}

// uploadMedia uploads the files to the set of u.device in u.locale of v and
// returns the planned or applied change and the resulting items (not read
// with dryRun).
func (s *Service) uploadMedia(ctx context.Context, k mediaKind, v Version, u mediaUpload) ([]Screenshot, metadata.Change, error) {
	ids, err := s.VersionLocalizationIDs(ctx, v)
	if err != nil {
		return nil, metadata.Change{}, err
	}
	locID, ok := ids[u.locale]
	if !ok {
		return nil, metadata.Change{}, noLocalization(v, u.locale)
	}
	change, setID, err := s.syncMedia(ctx, k, mediaSync{localizationID: locID, mediaUpload: u})
	items := []Screenshot{}
	if err != nil || u.dryRun || setID == "" {
		return items, change, err
	}
	if items, err = k.listItems(ctx, s.client, setID); err != nil {
		return []Screenshot{}, change, fmt.Errorf("list %s of %s %s: %w", k.plural, u.locale, u.device.Alias, err)
	}
	return items, change, nil
}

type mediaSync struct {
	localizationID string
	mediaUpload
}

// syncMedia returns the change and the ID of the set ("" when a dry run would
// create it). The files are already validated.
func (s *Service) syncMedia(ctx context.Context, k mediaKind, o mediaSync) (metadata.Change, string, error) {
	local, err := media.MD5List(o.paths)
	if err != nil {
		return metadata.Change{}, "", err
	}
	change := metadata.Change{Scope: k.scope, Locale: o.locale, Field: o.device.Alias}
	sets, err := k.listSets(ctx, s.client, o.localizationID, k.typeOf(o.device))
	if err != nil {
		return change, "", fmt.Errorf("list %s sets of %s: %w", k.name, o.locale, err)
	}
	var setID string
	var remote []Screenshot
	if len(sets) > 0 {
		setID = sets[0].id
		if remote, err = k.listItems(ctx, s.client, setID); err != nil {
			return change, setID, fmt.Errorf("list %s of %s %s: %w", k.plural, o.locale, o.device.Alias, err)
		}
	}
	change.Old = k.count(len(remote))

	// An item Apple rejected or still processes after an earlier failed upload
	// keeps its checksum, so only COMPLETE items count as done.
	remoteSums := make([]string, len(remote))
	allComplete := true
	for i, r := range remote {
		remoteSums[i] = r.Checksum
		allComplete = allComplete && r.State == string(asc.AppMediaAssetStateStateComplete)
	}
	switch {
	case o.replace && allComplete && slices.Equal(remoteSums, local):
		change.Action, change.New = metadata.ActionSkip, change.Old
		return change, setID, nil
	case o.replace:
		change.New = k.count(len(local))
	default:
		if total := len(remote) + len(local); total > k.max {
			return change, setID, clierr.New("%s %s already holds %d %s, adding %d would exceed the maximum of %d (use --replace)",
				o.locale, o.device.Alias, len(remote), k.plural, len(local), k.max)
		}
		change.New = k.count(len(remote) + len(local))
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
		if setID, err = k.createSet(ctx, s.client, o.localizationID, o.device); err != nil {
			return change, "", fmt.Errorf("create %s %s set of %s: %w", o.device.Alias, k.name, o.locale, err)
		}
	}
	if o.replace {
		for _, r := range remote {
			if err := k.deleteItem(ctx, s.client, r.ID); err != nil {
				return change, setID, fmt.Errorf("delete %s %s: %w", k.name, r.FileName, err)
			}
		}
	}
	for i, p := range o.paths {
		s.report.Progressf("%s %s: %s %d of %d", o.locale, o.device.Alias, k.name, i+1, len(o.paths))
		if _, err := s.uploadAsset(ctx, o.asset, setID, p, o.timeout); err != nil {
			return change, setID, err
		}
	}
	return change, setID, nil
}

// deleteMedia deletes the whole set of device d in locale of v and returns the
// planned or applied change and whether it deleted. A missing locale or set is
// an error. With dryRun it only reads.
func (s *Service) deleteMedia(ctx context.Context, k mediaKind, v Version, locale string, d media.Device, dryRun bool) (metadata.Change, bool, error) {
	ids, err := s.VersionLocalizationIDs(ctx, v)
	if err != nil {
		return metadata.Change{}, false, err
	}
	locID, ok := ids[locale]
	if !ok {
		return metadata.Change{}, false, noLocalization(v, locale)
	}
	sets, err := k.listSets(ctx, s.client, locID, k.typeOf(d))
	if err != nil {
		return metadata.Change{}, false, fmt.Errorf("list %s sets of %s: %w", k.name, locale, err)
	}
	if len(sets) == 0 {
		return metadata.Change{}, false, clierr.New("%s has no %s %s set", locale, d.Alias, k.name)
	}
	items, err := k.listItems(ctx, s.client, sets[0].id)
	if err != nil {
		return metadata.Change{}, false, fmt.Errorf("list %s of %s %s: %w", k.plural, locale, d.Alias, err)
	}
	change := metadata.Change{Action: ActionDelete, Scope: k.scope, Locale: locale, Field: d.Alias, Old: k.count(len(items))}
	if dryRun {
		return change, false, nil
	}
	if err := k.deleteSet(ctx, s.client, sets[0].id); err != nil {
		return change, false, fmt.Errorf("delete %s %s set of %s: %w", d.Alias, k.name, locale, err)
	}
	return change, true, nil
}
