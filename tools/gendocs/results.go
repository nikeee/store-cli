package main

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/nikeee/store-cli/internal/apple"
	"github.com/nikeee/store-cli/internal/cli"
	"github.com/nikeee/store-cli/internal/google"
	"github.com/nikeee/store-cli/internal/lint"
	"github.com/nikeee/store-cli/internal/metadata"
)

// resultType is one JSON result object. Name is what the Result line of a
// command help and the links in results.md use.
type resultType struct {
	Name string
	Type reflect.Type
	Note string
}

func rt[T any](name, note string) resultType {
	return resultType{name, reflect.TypeFor[T](), note}
}

// resultTypes is the one list of result objects that results.md documents.
// Every Result line of a command help names one of them (see main_test.go).
var resultTypes = []resultType{
	rt[apple.App]("App", ""),
	rt[apple.AppDetails]("AppDetails", "`versions` holds the newest versions per platform (at most 5)."),
	rt[cli.AppleAuthCheck]("AppleAuthCheck", "Never contains the key."),
	rt[apple.Build]("Build", "`state` is one of `processing`, `failed`, `invalid`, `valid`."),
	rt[apple.UploadResult]("BuildUpload", "`buildId` is empty when the command did not wait for the build."),
	rt[apple.BuildNotes]("BuildNotes", ""),
	rt[apple.Version]("Version", "`state` is App Store Connect's appVersionState; `build` is null when no build is attached."),
	rt[apple.VersionBuild]("VersionBuild", ""),
	rt[apple.Localization]("Localization", ""),
	rt[apple.LocaleResult]("LocaleResult", "`action` is `create`, `update`, `replace` or `skip` (`skip` when nothing had to change)."),
	rt[apple.ReleaseNote]("WhatsNew", ""),
	rt[apple.BetaGroup]("BetaGroup", ""),
	rt[apple.Tester]("Tester", ""),
	rt[apple.TesterRemoved]("TesterRemoved", "`action` is `delete`, or `skip` when the tester was not in the group."),
	rt[apple.GroupBuild]("GroupBuild", ""),
	rt[apple.BetaReview]("BetaReview", ""),
	rt[apple.SubmissionResult]("SubmissionResult", ""),
	rt[apple.PhasedRelease]("PhasedRelease", ""),
	rt[apple.SubmitResult]("SubmitResult", ""),
	rt[apple.Screenshot]("Screenshot", "Also the entry type of app previews."),
	rt[apple.ScreenshotSet]("ScreenshotSet", ""),
	rt[apple.ScreenshotsDeleted]("ScreenshotsDeleted", "`action` is `delete`, or `skip` when there was no set."),
	rt[apple.PreviewSet]("PreviewSet", ""),
	rt[apple.PreviewsDeleted]("PreviewsDeleted", "`action` is `delete`, or `skip` when there was no set."),
	rt[apple.ReviewInfo]("ReviewInfo", "`demoPassword` is `"+metadata.Mask+"` unless `--reveal` is given."),
	rt[apple.PullResult]("ApplePullResult", ""),
	rt[metadata.Plan]("Plan", "Used by `push` and by `--dry-run`."),
	rt[metadata.Change]("Change", "`action` is `create`, `update`, `replace` or `skip`."),
	rt[metadata.InitResult]("InitResult", ""),
	rt[lint.Result]("LintResult", "`errors` and `warnings` count the findings."),
	rt[lint.Finding]("Finding", "`severity` is `error` or `warning`; `file`, `line` and `col` are left out when unknown."),
	rt[cli.VersionInfo]("VersionInfo", "`dev` for local builds."),
	rt[cli.GoogleAuthCheck]("GoogleAuthCheck", ""),
	rt[google.BundleUploadResult]("BundleUploadResult", ""),
	rt[google.Track]("Track", ""),
	rt[google.Release]("Release", ""),
	rt[google.ReleaseNote]("ReleaseNote", ""),
	rt[google.Testers]("Testers", ""),
	rt[google.Listing]("Listing", ""),
	rt[google.Details]("Details", ""),
	rt[google.Image]("Image", ""),
	rt[google.ImageSet]("ImageSet", ""),
	rt[google.ImagesDeleted]("ImagesDeleted", "`action` is `delete`, or `skip` when there were no images; `deleted` is the number of removed images."),
	rt[google.PullResult]("GooglePullResult", ""),
}

func resultNames() map[string]bool {
	m := map[string]bool{}
	for _, r := range resultTypes {
		m[r.Name] = true
	}
	return m
}

// typeName describes a Go type the way it appears in JSON.
func typeName(t reflect.Type, known map[reflect.Type]string) string {
	if name, ok := known[t]; ok {
		return fmt.Sprintf("[%s](#%s)", name, strings.ToLower(name))
	}
	switch {
	case t == reflect.TypeFor[time.Time]():
		return "string (RFC 3339 time)"
	case t.Kind() == reflect.Pointer:
		return typeName(t.Elem(), known) + " or null"
	case t.Kind() == reflect.Slice:
		return "list of " + typeName(t.Elem(), known)
	}
	switch t.Kind() {
	case reflect.String:
		return "string"
	case reflect.Bool:
		return "bool"
	case reflect.Int, reflect.Int32, reflect.Int64:
		return "int"
	case reflect.Float32, reflect.Float64:
		return "number"
	}
	panic("gendocs: unsupported result field type " + t.String())
}

func resultsPage() string {
	known := map[reflect.Type]string{}
	for _, r := range resultTypes {
		known[r.Type] = r.Name
	}
	var b bytes.Buffer
	b.WriteString("# store - result objects\n\n")
	b.WriteString("Generated from the result types by `go generate ./...`; do not edit.\n")
	b.WriteString("With `--output json` (and `toon`) a command prints the object named in the `Result:` line of its help,\n")
	b.WriteString("with exactly these field names, in this order. `[Name]` in a `Result:` line is a list of that object.\n\n")
	for _, r := range resultTypes {
		fmt.Fprintf(&b, "## %s\n\n", r.Name)
		if r.Note != "" {
			b.WriteString(r.Note + "\n\n")
		}
		b.WriteString("| Field | Type |\n|---|---|\n")
		for i := range r.Type.NumField() {
			f := r.Type.Field(i)
			name, opts, _ := strings.Cut(f.Tag.Get("json"), ",")
			typ := typeName(f.Type, known)
			if opts == "omitempty" {
				typ += ", omitted when empty"
			}
			fmt.Fprintf(&b, "| `%s` | %s |\n", name, typ)
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}
