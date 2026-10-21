package apple

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/media"
	"github.com/nikeee/store-cli/internal/metadata"
)

// The parts of a push (values of PushOptions.Only).
const (
	partTexts       = "texts"
	partScreenshots = "screenshots"
	partReview      = "review"
)

// PushParts are the valid values of PushOptions.Only.
var PushParts = []string{partTexts, partScreenshots, partReview}

// nameFile is the metadata file of the app name, which cannot be cleared.
const nameFile = "name.txt"

// PushOptions selects what Push makes App Store Connect match.
type PushOptions struct {
	Dir            string   // metadata directory (fastlane layout)
	ScreenshotsDir string   // "" means <Dir>/../screenshots
	Version        string   // "" means the one editable version of Platform
	Platform       string   // --platform value: ios, macos, tvos or visionos
	Only           []string // parts to push (PushParts); empty means all
	DryRun         bool
}

// screenshotsRoot is the screenshots directory given, else <dir>/../screenshots.
func screenshotsRoot(dir, given string) string {
	if given != "" {
		return given
	}
	return filepath.Join(dir, "..", "screenshots")
}

// PushInput is a metadata directory and its screenshots, read and validated
// by ReadPushDir. Parts that are not pushed are empty.
type PushInput struct {
	opts    PushOptions
	locales map[string]metadata.Texts
	review  metadata.Texts
	shots   map[string]map[string][]string // locale -> device alias -> files
}

// ReadPushDir reads the directories of o and checks every text and image that
// is pushed. It works offline, so every problem shows before any API call.
func ReadPushDir(o PushOptions) (*PushInput, error) {
	for _, p := range o.Only {
		if !slices.Contains(PushParts, p) {
			return nil, clierr.Usagef("invalid --only %q (use %s)", p, strings.Join(PushParts, ", "))
		}
	}
	if fi, err := os.Stat(o.Dir); err != nil || !fi.IsDir() {
		return nil, clierr.New("metadata directory %s not found", o.Dir)
	}
	in := &PushInput{opts: o}
	locales, review, err := metadata.ReadAppleTexts(o.Dir)
	if err != nil {
		return nil, err
	}
	if in.pushes(partTexts) {
		in.locales = locales
	}
	if in.pushes(partReview) {
		in.review = review
	}
	if in.pushes(partScreenshots) {
		// A missing screenshots directory means no screenshots.
		if in.shots, err = media.ScanAppleScreenshots(screenshotsRoot(o.Dir, o.ScreenshotsDir), o.Platform); err != nil {
			return nil, err
		}
	}
	if err := in.validate(); err != nil {
		return nil, err
	}
	return in, nil
}

func (in *PushInput) pushes(part string) bool {
	return len(in.opts.Only) == 0 || slices.Contains(in.opts.Only, part)
}

// validate checks lengths and values of the texts and format, alpha channel,
// size and count of the screenshots. All problems are reported together.
func (in *PushInput) validate() error {
	var msgs []string
	add := func(err error) {
		if err != nil {
			msgs = append(msgs, err.Error())
		}
	}
	name, ok := metadata.FieldByFile(metadata.AppleFields, nameFile)
	if !ok {
		return clierr.New("internal error: no name field")
	}
	for _, l := range slices.Sorted(maps.Keys(in.locales)) {
		t := in.locales[l]
		add(metadata.ValidateLocaleTexts(l, metadata.AppleFields, t))
		// App Store Connect requires a name, so it cannot be cleared.
		if v, ok := t[name.API]; ok && v == "" {
			msgs = append(msgs, fmt.Sprintf("%s/%s: must not be empty, App Store Connect requires a name", l, name.File))
		}
	}
	add(metadata.ValidateLocaleTexts(metadata.ReviewDir, metadata.AppleReviewFields, in.review))
	for _, l := range slices.Sorted(maps.Keys(in.shots)) {
		for _, d := range media.Devices {
			if paths, ok := in.shots[l][d.Alias]; ok {
				add(CheckScreenshotFiles(d, paths))
			}
		}
	}
	if len(msgs) > 0 {
		return clierr.New("%s", strings.Join(msgs, "; "))
	}
	return nil
}

// PushPlan is a computed push: the plan to show and what ApplyPush sends.
type PushPlan struct {
	// Plan lists the changes in the order ApplyPush sends them, secrets masked.
	Plan metadata.Plan

	app     App
	version Version
	texts   []metadata.Change // app info level of every locale, then version level
	review  []metadata.Change
	shots   []shotSync
}

// shotSync is a screenshot set that ApplyPush replaces with paths.
type shotSync struct {
	change metadata.Change
	device media.Device
	paths  []string
}

// PlanPush resolves the target version (PushOptions.Version, else the one
// editable version) and compares the input with it: the texts per locale and
// level, the App Review information and the screenshots per locale and
// device. It only reads. Unchanged values are not part of the plan.
func (s *Service) PlanPush(ctx context.Context, app App, in *PushInput) (*PushPlan, error) {
	v, err := s.pushVersion(ctx, app, in.opts)
	if err != nil {
		return nil, err
	}
	p := &PushPlan{app: app, version: v}
	if p.texts, err = s.planTexts(ctx, app, v, in.locales); err != nil {
		return nil, err
	}
	if p.review, err = s.planReview(ctx, v, in.review); err != nil {
		return nil, err
	}
	if p.shots, err = s.planScreenshots(ctx, v, in.shots, p.texts); err != nil {
		return nil, err
	}
	changes := append([]metadata.Change{}, p.texts...)
	changes = append(changes, p.review...)
	for _, sh := range p.shots {
		changes = append(changes, sh.change)
	}
	p.Plan = metadata.Plan{Changes: metadata.MaskSecrets(metadata.AppleReviewFields, changes)}
	return p, nil
}

// pushVersion returns the version named in o, else the one editable version.
func (s *Service) pushVersion(ctx context.Context, app App, o PushOptions) (Version, error) {
	if o.Version == "" {
		return s.EditableVersion(ctx, app, o.Platform)
	}
	v, ok, err := s.FindVersion(ctx, app, o.Version, o.Platform)
	if err != nil {
		return Version{}, err
	}
	if !ok {
		e := clierr.New("version %s not found", o.Version)
		e.Hint = "store apple version create " + o.Version
		return Version{}, e
	}
	return v, nil
}

// planTexts diffs the local texts with the listing of v: the app info level
// of every locale first, then the version level, as ApplyTextChanges sends
// them. A locale missing on the store is created. What's New is left out
// with a warning while the app has never been released (Apple rejects it).
func (s *Service) planTexts(ctx context.Context, app App, v Version, local map[string]metadata.Texts) ([]metadata.Change, error) {
	if len(local) == 0 {
		return nil, nil
	}
	remote, err := s.ListingTexts(ctx, app, v)
	if err != nil {
		return nil, err
	}
	locales := slices.Sorted(maps.Keys(local))
	var changes []metadata.Change
	for _, level := range []metadata.Level{metadata.LevelAppInfo, metadata.LevelVersion} {
		for _, l := range locales {
			changes = append(changes, metadata.DiffTexts(level, l, metadata.AppleFields, local[l], trimmed(remote[l]))...)
		}
	}
	if changes, err = s.dropWhatsNewBeforeRelease(ctx, app, v, changes); err != nil {
		return nil, err
	}
	// App info texts change only while the app info is editable; checked here
	// so that a dry run reports it too.
	if slices.ContainsFunc(changes, func(c metadata.Change) bool { return c.Scope == string(metadata.LevelAppInfo) }) {
		if _, err := s.EditableAppInfo(ctx, app); err != nil {
			return nil, err
		}
	}
	return changes, nil
}

// dropWhatsNewBeforeRelease removes the What's New changes, with a warning,
// when app has never had a released version on the platform of v; App Store
// Connect accepts the field only after the first release (as in submit).
func (s *Service) dropWhatsNewBeforeRelease(ctx context.Context, app App, v Version, changes []metadata.Change) ([]metadata.Change, error) {
	f, ok := metadata.FieldByFile(metadata.AppleFields, releaseNotesFile)
	if !ok {
		return nil, clierr.New("internal error: no release notes field")
	}
	isWhatsNew := func(c metadata.Change) bool { return c.Field == f.API }
	if !slices.ContainsFunc(changes, isWhatsNew) {
		return changes, nil
	}
	released, err := s.HasReleasedVersion(ctx, app, v.Platform)
	if err != nil || released {
		return changes, err
	}
	s.report.Warnf("skipping What's New: App Store Connect accepts it only after the first release")
	return slices.DeleteFunc(changes, isWhatsNew), nil
}

// trimmed returns t with every value trimmed like a value read from a file,
// so that a remote value with trailing whitespace equals its pulled file; nil
// (no such localization) stays nil.
func trimmed(t metadata.Texts) metadata.Texts {
	if t == nil {
		return nil
	}
	out := make(metadata.Texts, len(t))
	for k, v := range t {
		out[k] = metadata.TrimValue(v)
	}
	return out
}

// planReview diffs the local App Review texts with those of v. An empty
// demo_required.txt means "no change", and on a version without review
// information an empty file has nothing to clear.
func (s *Service) planReview(ctx context.Context, v Version, local metadata.Texts) ([]metadata.Change, error) {
	if len(local) == 0 {
		return nil, nil
	}
	remote, err := s.ReviewTexts(ctx, v)
	if err != nil {
		return nil, err
	}
	remote = trimmed(remote)
	local = maps.Clone(local)
	maps.DeleteFunc(local, func(api, val string) bool { return val == "" && (api == apiDemoRequired || remote == nil) })
	return metadata.DiffTexts(metadata.LevelReview, "", metadata.AppleReviewFields, local, remote), nil
}

// planScreenshots compares the local screenshots per locale and device with
// the sets of v. A locale without a version localization needs a version
// text change that creates it; its sets are created.
func (s *Service) planScreenshots(ctx context.Context, v Version, local map[string]map[string][]string, texts []metadata.Change) ([]shotSync, error) {
	if len(local) == 0 {
		return nil, nil
	}
	ids, err := s.VersionLocalizationIDs(ctx, v)
	if err != nil {
		return nil, err
	}
	var out []shotSync
	for _, l := range slices.Sorted(maps.Keys(local)) {
		id, exists := ids[l]
		createdByTexts := slices.ContainsFunc(texts, func(c metadata.Change) bool {
			return c.Scope == string(metadata.LevelVersion) && c.Locale == l
		})
		if !exists && !createdByTexts {
			return nil, noLocalization(v, l)
		}
		for _, d := range media.Devices {
			paths, ok := local[l][d.Alias]
			if !ok {
				continue
			}
			change := metadata.Change{Action: metadata.ActionCreate, Scope: screenshotsScope, Locale: l, Field: d.Alias,
				Old: screenshotMedia.count(0), New: screenshotMedia.count(len(paths))}
			if exists {
				if change, err = s.SyncScreenshots(ctx, id, l, d, paths, true); err != nil {
					return nil, err
				}
			}
			if change.Action != metadata.ActionSkip {
				out = append(out, shotSync{change: change, device: d, paths: paths})
			}
		}
	}
	return out, nil
}

// changeKey identifies a change within a plan.
type changeKey struct{ scope, locale, field string }

func keyOf(c metadata.Change) changeKey { return changeKey{c.Scope, c.Locale, c.Field} }

// ApplyPush sends the changes of p in this order: app info texts, version
// texts, App Review information, screenshots. It returns the plan with the
// action of every applied change and skip for the others. On an error it
// stops; the error says how many changes were applied. A screenshot set that
// was changed before the error keeps its action and is reported incomplete.
func (s *Service) ApplyPush(ctx context.Context, p *PushPlan) (metadata.Plan, error) {
	applied, incomplete, err := s.applyPush(ctx, p)
	done := map[changeKey]bool{}
	for _, c := range applied {
		done[keyOf(c)] = true
	}
	out := metadata.Plan{Changes: make([]metadata.Change, len(p.Plan.Changes))}
	n := 0
	for i, c := range p.Plan.Changes {
		switch {
		case done[keyOf(c)]:
			n++
		case incomplete != nil && keyOf(c) == keyOf(*incomplete):
		default:
			c.Action = metadata.ActionSkip
		}
		out.Changes[i] = c
	}
	if err == nil {
		return out, nil
	}
	if incomplete != nil {
		return out, fmt.Errorf("applied %d of %d changes, 1 incomplete: %w", n, len(out.Changes), err)
	}
	return out, fmt.Errorf("applied %d of %d changes: %w", n, len(out.Changes), err)
}

// applyPush returns the changes applied, also on error, and the screenshot
// change that was started but not finished (nil if none).
func (s *Service) applyPush(ctx context.Context, p *PushPlan) (applied []metadata.Change, incomplete *metadata.Change, err error) {
	if len(p.texts) > 0 {
		a, err := s.ApplyTextChanges(ctx, p.app, p.version, p.texts)
		applied = append(applied, a...)
		if err != nil {
			return applied, nil, err
		}
	}
	if len(p.review) > 0 {
		a, err := s.ApplyReviewChanges(ctx, p.version, p.review)
		if err != nil {
			return applied, nil, err
		}
		applied = append(applied, a...)
	}
	if len(p.shots) == 0 {
		return applied, nil, nil
	}
	// Read again: the texts may have created localizations.
	ids, err := s.VersionLocalizationIDs(ctx, p.version)
	if err != nil {
		return applied, nil, err
	}
	for _, sh := range p.shots {
		c := sh.change
		id, ok := ids[c.Locale]
		if !ok {
			return applied, nil, noLocalization(p.version, c.Locale)
		}
		if _, err := s.SyncScreenshots(ctx, id, c.Locale, sh.device, sh.paths, false); err != nil {
			if errors.As(err, new(setChangedError)) {
				return applied, &c, fmt.Errorf("%s %s %s: set incomplete, run push again: %w", c.Scope, c.Locale, c.Field, err)
			}
			return applied, nil, err
		}
		applied = append(applied, c)
	}
	return applied, nil, nil
}

// Push makes App Store Connect match the metadata directory of o: it reads
// and checks the directory (ReadPushDir), computes the plan (PlanPush) and,
// unless o.DryRun, applies it (ApplyPush). The plan has secrets masked.
func (s *Service) Push(ctx context.Context, app App, o PushOptions) (metadata.Plan, error) {
	in, err := ReadPushDir(o)
	if err != nil {
		return metadata.Plan{}, err
	}
	p, err := s.PlanPush(ctx, app, in)
	if err != nil {
		return metadata.Plan{}, err
	}
	if o.DryRun {
		return p.Plan, nil
	}
	return s.ApplyPush(ctx, p)
}

// PullResult is the result of Pull.
type PullResult struct {
	Locales []string `json:"locales"`
	Files   int      `json:"files"`
}

// Pull writes the texts of every locale of v and its App Review information
// into the metadata directory dir. Secret fields (the demo password) are
// never written. It overwrites the files it writes and leaves all others
// alone; everything is read before the first file is written.
func (s *Service) Pull(ctx context.Context, app App, v Version, dir string) (PullResult, error) {
	listing, err := s.ListingTexts(ctx, app, v)
	if err != nil {
		return PullResult{}, err
	}
	review, err := s.ReviewTexts(ctx, v)
	if err != nil {
		return PullResult{}, err
	}
	res := PullResult{Locales: slices.Sorted(maps.Keys(listing))}
	if res.Locales == nil {
		res.Locales = []string{}
	}
	write := func(sub string, fields []metadata.Field, t metadata.Texts) error {
		written, err := metadata.WriteTexts(filepath.Join(dir, sub), fields, t)
		res.Files += len(written)
		return err
	}
	// A field the store has no value for is written empty, so a value cleared
	// in the store does not come back with the next push. The name is never
	// empty: without one there is no app info localization to mirror.
	clearable := slices.DeleteFunc(slices.Clone(metadata.AppleFields), func(f metadata.Field) bool { return f.File == nameFile })
	for _, l := range res.Locales {
		if err := write(l, metadata.AppleFields, withEmpty(clearable, listing[l])); err != nil {
			return res, err
		}
	}
	// Without review information on v the files are left alone.
	if review != nil {
		public := slices.DeleteFunc(slices.Clone(metadata.AppleReviewFields), func(f metadata.Field) bool { return f.Secret })
		if err := write(metadata.ReviewDir, public, withEmpty(public, review)); err != nil {
			return res, err
		}
	}
	return res, nil
}

// withEmpty returns a copy of t with "" for every field of fields it lacks.
func withEmpty(fields []metadata.Field, t metadata.Texts) metadata.Texts {
	out := maps.Clone(t)
	if out == nil {
		out = metadata.Texts{}
	}
	for _, f := range fields {
		if _, ok := out[f.API]; !ok {
			out[f.API] = ""
		}
	}
	return out
}

// Init scaffolds a metadata directory offline: per locale one empty file per
// AppleFields row, review_information/ with its files and
// <screenshotsDir>/<locale>/ (screenshotsDir "" means <dir>/../screenshots).
// Existing files and folders are left alone and reported in Existing. A
// locale that is not a locale folder name is a usage error; nothing is written
// then.
func Init(dir, screenshotsDir string, locales []string) (metadata.InitResult, error) {
	for _, l := range locales {
		if !metadata.IsLocale(l) {
			return metadata.InitResult{Created: []string{}, Existing: []string{}},
				clierr.Usagef("invalid --locale %q (use a locale like en-US, de-DE or zh-Hans)", l)
		}
	}
	res, err := metadata.Scaffold(dir, metadata.AppleFields, locales)
	if err != nil {
		return res, err
	}
	r, err := metadata.ScaffoldDir(filepath.Join(dir, metadata.ReviewDir), metadata.AppleReviewFields)
	res.Merge(r)
	if err != nil {
		return res, err
	}
	shots := screenshotsRoot(dir, screenshotsDir)
	if err := os.MkdirAll(shots, 0o755); err != nil {
		return res, clierr.New("create %s: %v", shots, err)
	}
	for _, l := range locales {
		p := filepath.Join(shots, l)
		switch err := os.Mkdir(p, 0o755); {
		case errors.Is(err, fs.ErrExist):
			res.Existing = append(res.Existing, p)
		case err != nil:
			return res, clierr.New("create %s: %v", p, err)
		default:
			res.Created = append(res.Created, p)
		}
	}
	return res, nil
}
