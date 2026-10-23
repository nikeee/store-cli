package apple

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/nikeee/store-cli/internal/apple/asc"
	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/metadata"
)

// Scopes and fields of the plan entries Submit adds to the metadata plan.
const (
	scopeVersion    = "version"
	scopeSubmission = "submission"

	stepCreateVersion = "create version"
	stepReleaseType   = "release type"
	stepAttachBuild   = "attach build"
	stepReleaseNotes  = "set release notes"
	stepSubmit        = "submit"
)

// SubmitOptions are the inputs of Submit. Build is a build number or
// "latest". MetadataDir, ScreenshotsDir and Notes (locale -> What's New, see
// metadata.ReadReleaseNotesDir) are optional. ReleaseType is an App Store
// Connect release type (see ParseReleaseType), empty to leave it as it is.
type SubmitOptions struct {
	Version        string
	Platform       string
	Build          string
	MetadataDir    string
	ScreenshotsDir string
	Notes          map[string]string
	ReleaseType    string
	ReleaseDate    *time.Time
	DryRun         bool
}

// SubmitResult is the outcome of Submit. Plan lists the steps with the
// metadata changes, secrets masked.
type SubmitResult struct {
	Version      string        `json:"version"`
	Build        string        `json:"build"`
	SubmissionID string        `json:"submissionId"`
	State        string        `json:"state"`
	Plan         metadata.Plan `json:"plan"`
}

func step(action metadata.Action, scope, locale, field, old, new string) metadata.Change {
	return metadata.Change{Action: action, Scope: scope, Locale: locale, Field: field, Old: old, New: new}
}

// progress tracks the steps of a submission: the plan in order, and which
// steps ran. A step is recorded as done only after its call succeeded.
type progress struct {
	steps []*submitStep
}

type submitStep struct {
	name    string            // for the error message
	changes []metadata.Change // one step may stand for several changes
	done    bool
	settled bool // changes carry their final actions (a partly applied push)
}

func (p *progress) add(name string, c ...metadata.Change) *submitStep {
	st := &submitStep{name: name, changes: c}
	p.steps = append(p.steps, st)
	return st
}

// plan returns all changes; with failed, those of steps that did not run are
// marked skip.
func (p *progress) plan(failed bool) []metadata.Change {
	var out []metadata.Change
	for _, st := range p.steps {
		for _, c := range st.changes {
			if failed && !st.done && !st.settled && c.Action != metadata.ActionSkip {
				c.Action = metadata.ActionSkip
			}
			out = append(out, c)
		}
	}
	return out
}

// describe names the steps done and not done, for an error message.
func (p *progress) describe() string {
	var done, todo []string
	for _, st := range p.steps {
		if len(st.changes) == 0 {
			continue // skipped, e.g. What's New before the first release
		}
		if st.done {
			done = append(done, st.name)
		} else {
			todo = append(todo, st.name)
		}
	}
	if len(done) == 0 {
		return ""
	}
	return fmt.Sprintf("done: %s; not done: %s; run the command again to continue", strings.Join(done, ", "), strings.Join(todo, ", "))
}

// CheckSubmitInputs reads and checks every local input of Submit, offline, so
// that problems show before any API call.
func CheckSubmitInputs(o SubmitOptions) error {
	if _, ok := APIPlatform(o.Platform); !ok {
		return clierr.Usagef("invalid --platform %q", o.Platform)
	}
	if err := CheckReleaseDate(o.ReleaseType, o.ReleaseDate); err != nil {
		return err
	}
	if o.MetadataDir != "" {
		if _, err := ReadPushDir(pushOptions(o)); err != nil {
			return err
		}
	}
	field, ok := metadata.FieldByFile(metadata.AppleFields, releaseNotesFile)
	if !ok {
		return clierr.New("internal error: no release notes field")
	}
	for _, l := range slices.Sorted(maps.Keys(o.Notes)) {
		if err := metadata.ValidateLocaleTexts(l, "", metadata.AppleFields, metadata.Texts{field.API: o.Notes[l]}); err != nil {
			return err
		}
	}
	return nil
}

// Submit does the whole App Store submission of o.Version: find the build,
// find or create the version (it must be editable), attach the build, set the
// release type if given, push the metadata directory, set What's New (skipped
// with a warning before the first release) and submit for review. Local
// inputs are checked before the first API call. With o.DryRun nothing is
// changed and State is "dry-run". When a step fails, the error names the
// steps done, and the returned plan marks the steps that did not run skip.
func (s *Service) Submit(ctx context.Context, app App, o SubmitOptions) (SubmitResult, error) {
	if err := CheckSubmitInputs(o); err != nil {
		return SubmitResult{}, err
	}
	b, err := s.submitBuild(ctx, app, o)
	if err != nil {
		return SubmitResult{}, err
	}
	v, found, err := s.FindVersion(ctx, app, o.Version, o.Platform)
	if err != nil {
		return SubmitResult{}, err
	}
	if found && !isEditable(v) {
		return SubmitResult{}, stateError("version %s is in state %s and cannot be submitted", v.Version, v.State)
	}
	// A version that does not exist yet is checked as it will be created.
	target := v
	if !found {
		target = Version{Platform: o.Platform, Version: o.Version, State: "PREPARE_FOR_SUBMISSION"}
	}
	if err := CheckSetVersionBuild(target, b); err != nil {
		return SubmitResult{}, err
	}

	var p progress
	var createStep, releaseStep *submitStep
	if !found {
		createStep = p.add("create version "+o.Version, step(metadata.ActionCreate, scopeVersion, "", stepCreateVersion, "", o.Version))
	} else if needsRelease(o, v) {
		releaseStep = p.add("set release type", step(metadata.ActionUpdate, scopeVersion, "", stepReleaseType,
			releaseValue(v.ReleaseType, v.EarliestReleaseDate), releaseValue(o.ReleaseType, o.ReleaseDate)))
	}
	old := ""
	if found && v.Build != nil {
		old = v.Build.Build
	}
	attachStep := p.add("attach build "+b.Build, step(metadata.ActionUpdate, scopeVersion, "", stepAttachBuild, old, b.Build))
	var pushStep *submitStep
	if o.MetadataDir != "" {
		pushStep = p.add("push metadata", step(metadata.ActionCreate, scopeVersion, "", "push metadata", "", filepath.Base(o.MetadataDir)))
	}
	var notesStep *submitStep
	if len(o.Notes) > 0 {
		notesStep = p.add("set release notes", s.notesChanges(o)...)
	}
	submitStepP := p.add("submit", step(metadata.ActionCreate, scopeSubmission, "", stepSubmit, "", o.Version))

	fail := func(err error) (SubmitResult, error) {
		res := SubmitResult{Version: o.Version, Build: b.Build, Plan: s.maskedPlan(p.plan(true))}
		if d := p.describe(); d != "" {
			err = fmt.Errorf("%s: %w", d, err)
		}
		return res, err
	}

	if o.DryRun {
		if !found {
			if v, err = s.CreateVersion(ctx, app, s.createOptions(o)); err != nil {
				return SubmitResult{}, err
			}
		}
	} else {
		if v, err = s.submitPrepareVersion(ctx, app, o, v, found); err != nil {
			return fail(err)
		}
		for _, st := range []*submitStep{createStep, releaseStep} {
			if st != nil {
				st.done = true
			}
		}
		if v, err = s.SetVersionBuild(ctx, v, b); err != nil {
			return fail(err)
		}
		attachStep.done = true
	}

	if pushStep != nil {
		if found || !o.DryRun {
			pushed, err := s.Push(ctx, app, pushOptions(o))
			// Push reports its own changes, with skip for those not applied.
			pushStep.changes = pushed.Changes
			if err != nil {
				pushStep.settled = true
				return fail(err)
			}
		}
		pushStep.done = true
	}
	if notesStep != nil {
		skipped, err := s.submitReleaseNotes(ctx, app, o, v, found || !o.DryRun)
		if err != nil {
			return fail(err)
		}
		if skipped {
			notesStep.changes = nil
		}
		notesStep.done = true
	}

	submit := s.SubmitVersion
	if o.DryRun {
		submit = s.PreviewSubmitVersion
	}
	sub, err := submit(ctx, app, v)
	if err != nil {
		return fail(err)
	}
	submitStepP.done = true
	return SubmitResult{Version: o.Version, Build: b.Build, SubmissionID: sub.SubmissionID, State: sub.State, Plan: s.maskedPlan(p.plan(false))}, nil
}

func (s *Service) maskedPlan(changes []metadata.Change) metadata.Plan {
	return metadata.Plan{Changes: metadata.MaskSecrets(metadata.AppleReviewFields, changes)}
}

func pushOptions(o SubmitOptions) PushOptions {
	return PushOptions{Dir: o.MetadataDir, ScreenshotsDir: o.ScreenshotsDir, Version: o.Version, Platform: o.Platform, DryRun: o.DryRun}
}

func (s *Service) createOptions(o SubmitOptions) CreateVersionOptions {
	return CreateVersionOptions{Version: o.Version, Platform: o.Platform, ReleaseType: o.ReleaseType, ReleaseDate: o.ReleaseDate, DryRun: o.DryRun}
}

// submitBuild resolves o.Build, which must be a valid build of o.Version;
// "latest" (the default) is the newest valid, unexpired build of o.Version.
func (s *Service) submitBuild(ctx context.Context, app App, o SubmitOptions) (Build, error) {
	if o.Build != "" && o.Build != "latest" {
		return s.ResolveBuild(ctx, app, o.Build, o.Version, o.Platform)
	}
	builds, err := s.ListBuilds(ctx, app, BuildFilter{Version: o.Version, State: "valid", Limit: 1, Platform: o.Platform, ExcludeExpired: true})
	if err != nil {
		return Build{}, err
	}
	if len(builds) == 0 {
		e := clierr.New("no valid build found for version %s", o.Version)
		e.Hint = "store apple build list --version " + o.Version
		return Build{}, e
	}
	return builds[0], nil
}

// needsRelease reports whether the existing version v must get the release
// type or date o asks for.
func needsRelease(o SubmitOptions, v Version) bool {
	if o.ReleaseType == "" {
		return false
	}
	if o.ReleaseType != v.ReleaseType {
		return true
	}
	return o.ReleaseDate != nil && (v.EarliestReleaseDate == nil || !o.ReleaseDate.Equal(*v.EarliestReleaseDate))
}

// releaseValue renders a release type and its date for the plan.
func releaseValue(releaseType string, date *time.Time) string {
	if date == nil {
		return releaseType
	}
	return releaseType + " " + date.UTC().Format(time.RFC3339)
}

// submitPrepareVersion creates the version if it does not exist, else sets
// its release type or date when o asks for other ones.
func (s *Service) submitPrepareVersion(ctx context.Context, app App, o SubmitOptions, v Version, found bool) (Version, error) {
	if !found {
		return s.CreateVersion(ctx, app, s.createOptions(o))
	}
	if !needsRelease(o, v) {
		return v, nil
	}
	attrs := asc.AppStoreVersionUpdateRequestDataAttributes{
		ReleaseType: asc.Value(asc.AppStoreVersionUpdateRequestDataAttributesReleaseType(o.ReleaseType)),
	}
	if o.ReleaseDate != nil {
		attrs.EarliestReleaseDate = asc.Value(*o.ReleaseDate)
	}
	_, err := s.client.AppStoreVersionsUpdateInstance(ctx, asc.AppStoreVersionsUpdateInstanceParams{ID: v.ID}, asc.AppStoreVersionUpdateRequest{
		Data: asc.AppStoreVersionUpdateRequestData{Type: asc.ResourceTypeAppStoreVersions, ID: v.ID, Attributes: &attrs},
	})
	if err != nil {
		return Version{}, fmt.Errorf("set release type of version %s: %w", v.Version, err)
	}
	v.ReleaseType = o.ReleaseType
	v.EarliestReleaseDate = o.ReleaseDate
	return v, nil
}

// notesChanges are the plan entries of the What's New step, one per locale.
func (s *Service) notesChanges(o SubmitOptions) []metadata.Change {
	out := make([]metadata.Change, 0, len(o.Notes))
	for _, l := range slices.Sorted(maps.Keys(o.Notes)) {
		out = append(out, step(metadata.ActionUpdate, scopeVersion, l, stepReleaseNotes, "", o.Notes[l]))
	}
	return out
}

// submitReleaseNotes sets What's New per locale, or skips it with a warning
// before the first release. exists is false in a dry run for a version that
// does not exist yet.
func (s *Service) submitReleaseNotes(ctx context.Context, app App, o SubmitOptions, v Version, exists bool) (skipped bool, err error) {
	switch {
	case !exists:
		var released bool
		if released, err = s.HasReleasedVersion(ctx, app, o.Platform); err == nil && !released {
			err = notYetReleased()
		}
	case o.DryRun:
		_, err = s.PreviewReleaseNotes(ctx, app, v, o.Notes)
	default:
		_, err = s.SetReleaseNotes(ctx, app, v, o.Notes)
	}
	var ce *clierr.Error
	if errors.As(err, &ce) && ce.Code == CodeNotYetReleased {
		s.report.Warnf("skipping What's New: App Store Connect accepts it only after the first release")
		return true, nil
	}
	return false, err
}
