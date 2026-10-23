package google

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/locale"
	"github.com/nikeee/store-cli/internal/media"
	"github.com/nikeee/store-cli/internal/metadata"
)

// The parts of a push (values of PushOptions.Only).
const (
	partTexts  = "texts"
	partImages = "images"
)

// PushParts are the valid values of PushOptions.Only.
var PushParts = []string{partTexts, partImages}

// PushOptions selects what Push makes the Play listing match.
type PushOptions struct {
	Dir  string   // metadata directory (fastlane layout); the listing is in <Dir>/android/
	Only []string // parts to push (PushParts); empty means all
}

// PushInput is the listing directory read and validated by ReadPushDir,
// keyed by Play language. Parts that are not pushed are empty.
type PushInput struct {
	opts    PushOptions
	locales map[string]metadata.Texts
	images  map[string]map[string][]string // language -> image type alias -> files
}

// ReadPushDir reads <dir>/android/ and checks every text and image that is
// pushed. It works offline, so every problem shows before any API call.
func ReadPushDir(o PushOptions) (*PushInput, error) {
	in, err := readPushDir(o)
	if err != nil {
		return nil, err
	}
	var c clierr.Collector
	in.validate(&c)
	if err := c.Err(); err != nil {
		return nil, err
	}
	return in, nil
}

// readPushDir reads <dir>/android/ without checking the texts and images.
func readPushDir(o PushOptions) (*PushInput, error) {
	for _, p := range o.Only {
		if !slices.Contains(PushParts, p) {
			return nil, clierr.Usagef("invalid --only %q (use %s)", p, strings.Join(PushParts, ", "))
		}
	}
	root := filepath.Join(o.Dir, metadata.AndroidDir)
	if fi, err := os.Stat(root); err != nil || !fi.IsDir() {
		return nil, clierr.New("listing directory %s not found", root)
	}
	in := &PushInput{opts: o}
	var err error
	if in.pushes(partTexts) {
		if in.locales, err = metadata.ReadGoogleTexts(o.Dir); err != nil {
			return nil, err
		}
	}
	if in.pushes(partImages) {
		if in.images, err = media.ScanGoogleImages(o.Dir); err != nil {
			return nil, err
		}
	}
	return in, nil
}

func (in *PushInput) pushes(part string) bool {
	return len(in.opts.Only) == 0 || slices.Contains(in.opts.Only, part)
}

// validate adds to c the problems of the folder names, the lengths of the
// texts and the number, format, size and aspect ratio of the images.
func (in *PushInput) validate(c *clierr.Collector) {
	folders := slices.Concat(slices.Collect(maps.Keys(in.locales)), slices.Collect(maps.Keys(in.images)))
	slices.Sort(folders)
	for _, l := range slices.Compact(folders) {
		if p := locale.ToPlay(l); p != l {
			c.Add(clierr.New("%s: Google Play calls this language %s, rename the folder", l, p))
		}
	}
	for _, l := range slices.Sorted(maps.Keys(in.locales)) {
		dir := filepath.Join(in.opts.Dir, metadata.AndroidDir, l)
		c.Add(metadata.ValidateLocaleTexts(l, dir, metadata.GoogleFields, in.locales[l]))
	}
	for _, l := range slices.Sorted(maps.Keys(in.images)) {
		for _, t := range media.ImageTypes {
			// push makes each set exactly these files
			if paths, ok := in.images[l][t.Alias]; ok {
				if err := CheckImageFiles(t, paths, true); err != nil {
					c.Add(fmt.Errorf("%s: %w", l, err))
				}
			}
		}
	}
}

// pushPlan is a computed push: the plan to show and what applyPush sends.
type pushPlan struct {
	// plan lists the changes in the order applyPush stages them.
	plan   metadata.Plan
	texts  []metadata.Change
	images []imageSync
}

// imageSync is an image set that applyPush makes exactly paths.
type imageSync struct {
	change metadata.Change
	typ    media.ImageType
	paths  []string
}

// planPush compares the input with the listing of the edit: the texts per
// language, then the images per language and type. It only reads. Unchanged
// values are not part of the plan.
func (s *Service) planPush(ctx context.Context, editID string, in *PushInput) (*pushPlan, error) {
	remote, err := s.ListingTexts(ctx, editID)
	if err != nil {
		return nil, err
	}
	p := &pushPlan{texts: planTexts(in.locales, remote)}
	if p.images, err = s.planImages(ctx, editID, in.images, remote, p.texts); err != nil {
		return nil, err
	}
	changes := append([]metadata.Change{}, p.texts...)
	for _, im := range p.images {
		changes = append(changes, im.change)
	}
	p.plan = metadata.Plan{Changes: changes}
	return p, nil
}

// planTexts diffs the local texts with the listings, language by language.
// Remote values are trimmed like a file's, so that a pulled file equals its
// value. A language without a listing is created; its empty files have
// nothing to clear and are left out.
func planTexts(local, remote map[string]metadata.Texts) []metadata.Change {
	var changes []metadata.Change
	for _, l := range slices.Sorted(maps.Keys(local)) {
		t, r := local[l], trimmed(remote[l])
		if r == nil {
			t = maps.Clone(t)
			maps.DeleteFunc(t, func(_, v string) bool { return v == "" })
		}
		changes = append(changes, metadata.DiffTexts(metadata.LevelListing, l, metadata.GoogleFields, t, r)...)
	}
	return changes
}

// trimmed returns t with every value trimmed like a value read from a file;
// nil (no such listing) stays nil.
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

// planImages compares the local images per language and type with the sets
// of the edit. A language without a listing needs a text change that creates
// it; its sets are created.
func (s *Service) planImages(ctx context.Context, editID string, local map[string]map[string][]string, remote map[string]metadata.Texts, texts []metadata.Change) ([]imageSync, error) {
	var out []imageSync
	for _, l := range slices.Sorted(maps.Keys(local)) {
		_, exists := remote[l]
		if !exists && !slices.ContainsFunc(texts, func(c metadata.Change) bool { return c.Locale == l }) {
			e := clierr.New("no listing for %s: its images need one, add texts for %s to create it", l, l)
			e.Hint = "store google listing set --locale " + l + " --title <title>"
			return nil, e
		}
		for _, t := range media.ImageTypes {
			paths, ok := local[l][t.Alias]
			if !ok {
				continue
			}
			change := metadata.Change{Action: metadata.ActionCreate, Scope: scopeImages, Locale: l, Field: t.Alias,
				Old: imageCount(0), New: imageCount(len(paths))}
			if exists {
				var err error
				if change, err = s.SyncImages(ctx, editID, l, t, paths, true); err != nil {
					return nil, err
				}
			}
			if change.Action != metadata.ActionSkip {
				out = append(out, imageSync{change: change, typ: t, paths: paths})
			}
		}
	}
	return out, nil
}

// applyPush stages the changes of p in the edit in plan order: the listings,
// then the images. It returns how many changes were staged, also on error.
func (s *Service) applyPush(ctx context.Context, editID string, p *pushPlan) (int, error) {
	staged := 0
	if len(p.texts) > 0 {
		applied, err := s.ApplyListingChanges(ctx, editID, p.texts, nil)
		staged += len(applied)
		if err != nil {
			return staged, err
		}
	}
	for _, im := range p.images {
		if _, err := s.SyncImages(ctx, editID, im.change.Locale, im.typ, im.paths, false); err != nil {
			return staged, err
		}
		staged++
	}
	return staged, nil
}

// Push makes the Play listing match in, all in one edit: it computes the plan
// and hands it to onPlan before anything changes, then, unless c.DryRun,
// stages the listings and the images and commits the edit. With nothing to
// change the edit is deleted without a commit. On any failure the edit is
// deleted, so nothing is published; the error says so and how many changes
// had been staged, and the returned plan keeps the action of the staged
// changes and has skip for the others. A commit without an answer from Play
// (ErrCommitUnknown) is not said to have published nothing.
func (s *Service) Push(ctx context.Context, in *PushInput, c CommitOptions, onPlan func(metadata.Plan)) (metadata.Plan, error) {
	var (
		p      *pushPlan
		staged int
	)
	err := s.withEdit(ctx, c, func(editID string) error {
		var err error
		if p, err = s.planPush(ctx, editID, in); err != nil {
			return err
		}
		onPlan(p.plan)
		switch {
		case len(p.plan.Changes) == 0:
			return errNoChange
		case c.DryRun:
			return nil
		}
		staged, err = s.applyPush(ctx, editID, p)
		return err
	})
	switch {
	case err == nil || errors.Is(err, errNoChange):
		return p.plan, nil
	case p == nil:
		return metadata.Plan{}, fmt.Errorf("nothing was published: %w", err)
	}
	// applyPush stages in plan order: the staged changes come first.
	res := metadata.Plan{Changes: slices.Clone(p.plan.Changes)}
	for i := staged; i < len(res.Changes); i++ {
		res.Changes[i].Action = metadata.ActionSkip
	}
	if errors.Is(err, ErrCommitUnknown) {
		return res, fmt.Errorf("staged %d of %d changes: %w", staged, len(res.Changes), err)
	}
	return res, fmt.Errorf("staged %d of %d changes, then the edit was discarded, nothing was published: %w", staged, len(res.Changes), err)
}

// PullResult is the result of Pull.
type PullResult struct {
	Locales []string `json:"locales"`
	Files   int      `json:"files"`
}

// Pull writes the texts of every listing language into
// <dir>/android/<language>/. A field without a value in Play is written as an
// empty file, so a value cleared in Play does not come back with the next
// push. It overwrites the files it writes and leaves all others alone;
// everything is read before the first file is written. Images are not pulled.
func (s *Service) Pull(ctx context.Context, dir string) (PullResult, error) {
	var listings map[string]metadata.Texts
	err := s.withEdit(ctx, readOnly, func(editID string) error {
		var err error
		listings, err = s.ListingTexts(ctx, editID)
		return err
	})
	if err != nil {
		return PullResult{}, err
	}
	res := PullResult{Locales: []string{}}
	for _, l := range slices.Sorted(maps.Keys(listings)) {
		// The language becomes a folder name: never one like "../x".
		if !metadata.IsLocale(l) {
			s.report.Warnf("skipping Play language %q: not a locale name", l)
			continue
		}
		res.Locales = append(res.Locales, l)
	}
	for _, l := range res.Locales {
		t := maps.Clone(listings[l])
		for _, f := range metadata.GoogleFields {
			if _, ok := t[f.API]; !ok {
				t[f.API] = ""
			}
		}
		written, err := metadata.WriteTexts(filepath.Join(dir, metadata.AndroidDir, l), metadata.GoogleFields, t)
		res.Files += len(written)
		if err != nil {
			return res, err
		}
	}
	return res, nil
}

// Init scaffolds <dir>/android/ offline: per locale (App Store style or Play
// code, written as the Play code) one empty file per GoogleFields row and the
// image folders of media.ImageTypes. Existing files and folders are left
// alone and reported in Existing. A locale that is not a locale folder name is
// a usage error; nothing is written then.
func Init(dir string, locales []string) (metadata.InitResult, error) {
	res := metadata.InitResult{Created: []string{}, Existing: []string{}}
	var languages []string
	for _, l := range locales {
		if !metadata.IsLocale(l) {
			return res, clierr.Usagef("invalid --locale %q (use a locale like en-US, de-DE or es-419)", l)
		}
		if p := locale.ToPlay(l); !slices.Contains(languages, p) {
			languages = append(languages, p)
		}
	}
	for _, l := range languages {
		ldir := filepath.Join(dir, metadata.AndroidDir, l)
		r, err := metadata.ScaffoldDir(ldir, metadata.GoogleFields)
		res.Merge(r)
		if err != nil {
			return res, err
		}
		for _, f := range imageFolders() {
			p := filepath.Join(ldir, filepath.FromSlash(f))
			switch err := os.Mkdir(p, 0o755); {
			case errors.Is(err, fs.ErrExist):
				res.Existing = append(res.Existing, p)
			case err != nil:
				return res, clierr.New("%v", err)
			default:
				res.Created = append(res.Created, p)
			}
		}
	}
	return res, nil
}

// imageFolders returns the folders below a locale folder that hold the images
// of media.ImageTypes (a single image lies in the folder of its file), sorted
// so that a folder comes before its subfolders.
func imageFolders() []string {
	var out []string
	for _, t := range media.ImageTypes {
		f := t.Path
		if !t.Multiple {
			f = path.Dir(f)
		}
		if !slices.Contains(out, f) {
			out = append(out, f)
		}
	}
	slices.Sort(out)
	return out
}
