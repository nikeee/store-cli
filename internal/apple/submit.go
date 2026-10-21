package apple

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
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

// Submit does the whole App Store submission of o.Version: find the build,
// find or create the version (it must be editable), attach the build, set the
// release type if given, push the metadata directory, set What's New (skipped
// with a warning before the first release) and submit for review. Local
// inputs are checked before the first API call. With o.DryRun nothing is
// changed and State is "dry-run".
func (s *Service) Submit(ctx context.Context, app App, o SubmitOptions) (SubmitResult, error) {
	if o.ReleaseType == "SCHEDULED" && o.ReleaseDate == nil {
		return SubmitResult{}, clierr.Usagef("--release scheduled needs --release-date")
	}
	if err := s.checkSubmitInputs(o); err != nil {
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
	var plan []metadata.Change
	switch {
	case !found:
		plan = append(plan, step(metadata.ActionCreate, scopeVersion, "", stepCreateVersion, "", o.Version))
	case !isEditable(v):
		return SubmitResult{}, stateError("version %s is in state %s and cannot be submitted", v.Version, v.State)
	}
	if found && o.ReleaseType != "" && o.ReleaseType != v.ReleaseType {
		plan = append(plan, step(metadata.ActionUpdate, scopeVersion, "", stepReleaseType, v.ReleaseType, o.ReleaseType))
	}
	// A version that does not exist yet is checked as it will be created.
	target := v
	if !found {
		target = Version{Platform: o.Platform, Version: o.Version, State: "PREPARE_FOR_SUBMISSION"}
	}
	if err := CheckSetVersionBuild(target, b); err != nil {
		return SubmitResult{}, err
	}
	old := ""
	if found && v.Build != nil {
		old = v.Build.Build
	}
	plan = append(plan, step(metadata.ActionUpdate, scopeVersion, "", stepAttachBuild, old, b.Build))

	if !o.DryRun {
		if v, err = s.submitPrepareVersion(ctx, app, o, v, found); err != nil {
			return SubmitResult{}, err
		}
		if v, err = s.SetVersionBuild(ctx, v, b); err != nil {
			return SubmitResult{}, err
		}
	} else if !found {
		if v, err = s.CreateVersion(ctx, app, s.createOptions(o)); err != nil {
			return SubmitResult{}, err
		}
	}

	pushed, err := s.submitMetadata(ctx, app, o, found || !o.DryRun)
	plan = append(plan, pushed.Changes...)
	if err != nil {
		return SubmitResult{Version: o.Version, Build: b.Build, Plan: s.maskedPlan(plan)}, err
	}
	notes, err := s.submitReleaseNotes(ctx, app, o, v, found || !o.DryRun)
	if err != nil {
		return SubmitResult{Version: o.Version, Build: b.Build, Plan: s.maskedPlan(plan)}, err
	}
	plan = append(plan, notes...)

	submit := s.SubmitVersion
	if o.DryRun {
		submit = s.PreviewSubmitVersion
	}
	sub, err := submit(ctx, app, v)
	plan = append(plan, step(metadata.ActionCreate, scopeSubmission, "", stepSubmit, "", o.Version))
	res := SubmitResult{Version: o.Version, Build: b.Build, SubmissionID: sub.SubmissionID, State: sub.State, Plan: s.maskedPlan(plan)}
	return res, err
}

func (s *Service) maskedPlan(changes []metadata.Change) metadata.Plan {
	return metadata.Plan{Changes: metadata.MaskSecrets(metadata.AppleReviewFields, changes)}
}

// checkSubmitInputs reads and checks the local inputs, offline.
func (s *Service) checkSubmitInputs(o SubmitOptions) error {
	if _, ok := APIPlatform(o.Platform); !ok {
		return clierr.Usagef("invalid --platform %q", o.Platform)
	}
	if o.MetadataDir != "" {
		if _, err := ReadPushDir(s.pushOptions(o)); err != nil {
			return err
		}
	}
	field, ok := metadata.FieldByFile(metadata.AppleFields, releaseNotesFile)
	if !ok {
		return clierr.New("internal error: no release notes field")
	}
	for _, l := range slices.Sorted(maps.Keys(o.Notes)) {
		if err := metadata.ValidateLocaleTexts(l, metadata.AppleFields, metadata.Texts{field.API: o.Notes[l]}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) pushOptions(o SubmitOptions) PushOptions {
	return PushOptions{Dir: o.MetadataDir, ScreenshotsDir: o.ScreenshotsDir, Version: o.Version, Platform: o.Platform, DryRun: o.DryRun}
}

func (s *Service) createOptions(o SubmitOptions) CreateVersionOptions {
	return CreateVersionOptions{Version: o.Version, Platform: o.Platform, ReleaseType: o.ReleaseType, ReleaseDate: o.ReleaseDate, DryRun: o.DryRun}
}

// submitBuild resolves o.Build, which must be a valid build of o.Version.
func (s *Service) submitBuild(ctx context.Context, app App, o SubmitOptions) (Build, error) {
	ref := o.Build
	if ref == "" {
		ref = "latest"
	}
	b, err := s.ResolveBuild(ctx, app, ref, o.Version, o.Platform)
	if err == nil && ref == "latest" && b.State != "valid" {
		e := clierr.New("the latest build %s of version %s is %s, not valid", b.Build, o.Version, b.State)
		e.Hint = "store apple build list --version " + o.Version
		return Build{}, e
	}
	var ce *clierr.Error
	if errors.As(err, &ce) && ref == "latest" && ce.Hint == "store apple build list" {
		ce.Hint = "store apple build list --version " + o.Version
	}
	return b, err
}

// submitPrepareVersion creates the version if it does not exist, else sets
// its release type when o asks for another one.
func (s *Service) submitPrepareVersion(ctx context.Context, app App, o SubmitOptions, v Version, found bool) (Version, error) {
	if !found {
		return s.CreateVersion(ctx, app, s.createOptions(o))
	}
	if o.ReleaseType == "" || (o.ReleaseType == v.ReleaseType && o.ReleaseDate == nil) {
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

// submitMetadata pushes the metadata directory. exists is false in a dry run
// for a version that does not exist yet, which push cannot compare with.
func (s *Service) submitMetadata(ctx context.Context, app App, o SubmitOptions, exists bool) (metadata.Plan, error) {
	if o.MetadataDir == "" {
		return metadata.Plan{}, nil
	}
	if !exists {
		return metadata.Plan{Changes: []metadata.Change{step(metadata.ActionCreate, scopeVersion, "", "push metadata", "", filepath.Base(o.MetadataDir))}}, nil
	}
	return s.Push(ctx, app, s.pushOptions(o))
}

// submitReleaseNotes sets What's New per locale, or skips it with a warning
// before the first release. exists is as for submitMetadata.
func (s *Service) submitReleaseNotes(ctx context.Context, app App, o SubmitOptions, v Version, exists bool) ([]metadata.Change, error) {
	if len(o.Notes) == 0 {
		return nil, nil
	}
	var err error
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
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := make([]metadata.Change, 0, len(o.Notes))
	for _, l := range slices.Sorted(maps.Keys(o.Notes)) {
		out = append(out, step(metadata.ActionUpdate, scopeVersion, l, stepReleaseNotes, "", o.Notes[l]))
	}
	return out, nil
}
