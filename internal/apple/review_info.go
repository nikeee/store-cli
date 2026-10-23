package apple

import (
	"context"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/nikeee/store-cli/internal/apple/asc"
	"github.com/nikeee/store-cli/internal/apple/ascclient"
	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/metadata"
)

// ReviewInfo is the App Review information of a version.
type ReviewInfo struct {
	FirstName    string   `json:"firstName"`
	LastName     string   `json:"lastName"`
	Phone        string   `json:"phone"`
	Email        string   `json:"email"`
	DemoRequired bool     `json:"demoRequired"`
	DemoUser     string   `json:"demoUser"`
	DemoPassword string   `json:"demoPassword"`
	Notes        string   `json:"notes"`
	Attachments  []string `json:"attachments"`
}

const (
	apiDemoRequired = "demoAccountRequired"
	apiDemoPassword = "demoAccountPassword"
	scopeReview     = string(metadata.LevelReview)
	// fieldAttachments is the Field of the attachments Change.
	fieldAttachments = "attachments"
)

// reviewField selects one attribute of an App Review detail. It is the only
// place that knows the generated field names; read (ReviewTexts) and write
// (ApplyReviewChanges) both use it. write sets an update request attribute;
// the create request has identical fields, so it is converted from it.
type reviewField struct {
	read  func(*asc.AppStoreReviewDetailAttributes) (string, bool)
	write func(*asc.AppStoreReviewDetailUpdateRequestDataAttributes, string)
}

func stringRead(get func(*asc.AppStoreReviewDetailAttributes) *string) func(*asc.AppStoreReviewDetailAttributes) (string, bool) {
	return func(a *asc.AppStoreReviewDetailAttributes) (string, bool) {
		if p := get(a); p != nil {
			return *p, true
		}
		return "", false
	}
}

var reviewFields = map[string]reviewField{
	"contactFirstName": {
		read: stringRead(func(a *asc.AppStoreReviewDetailAttributes) *string { return a.ContactFirstName }),
		write: func(a *asc.AppStoreReviewDetailUpdateRequestDataAttributes, v string) {
			a.ContactFirstName = nullable(v)
		},
	},
	"contactLastName": {
		read: stringRead(func(a *asc.AppStoreReviewDetailAttributes) *string { return a.ContactLastName }),
		write: func(a *asc.AppStoreReviewDetailUpdateRequestDataAttributes, v string) {
			a.ContactLastName = nullable(v)
		},
	},
	"contactPhone": {
		read:  stringRead(func(a *asc.AppStoreReviewDetailAttributes) *string { return a.ContactPhone }),
		write: func(a *asc.AppStoreReviewDetailUpdateRequestDataAttributes, v string) { a.ContactPhone = nullable(v) },
	},
	"contactEmail": {
		read:  stringRead(func(a *asc.AppStoreReviewDetailAttributes) *string { return a.ContactEmail }),
		write: func(a *asc.AppStoreReviewDetailUpdateRequestDataAttributes, v string) { a.ContactEmail = nullable(v) },
	},
	apiDemoRequired: {
		read: func(a *asc.AppStoreReviewDetailAttributes) (string, bool) {
			if a.DemoAccountRequired == nil {
				return "", false
			}
			return strconv.FormatBool(*a.DemoAccountRequired), true
		},
		// v is "true" or "false" (validated); "" never reaches here.
		write: func(a *asc.AppStoreReviewDetailUpdateRequestDataAttributes, v string) {
			a.DemoAccountRequired = asc.Value(v == "true")
		},
	},
	"demoAccountName": {
		read: stringRead(func(a *asc.AppStoreReviewDetailAttributes) *string { return a.DemoAccountName }),
		write: func(a *asc.AppStoreReviewDetailUpdateRequestDataAttributes, v string) {
			a.DemoAccountName = nullable(v)
		},
	},
	apiDemoPassword: {
		read: stringRead(func(a *asc.AppStoreReviewDetailAttributes) *string { return a.DemoAccountPassword }),
		write: func(a *asc.AppStoreReviewDetailUpdateRequestDataAttributes, v string) {
			a.DemoAccountPassword = nullable(v)
		},
	},
	"notes": {
		read:  stringRead(func(a *asc.AppStoreReviewDetailAttributes) *string { return a.Notes }),
		write: func(a *asc.AppStoreReviewDetailUpdateRequestDataAttributes, v string) { a.Notes = nullable(v) },
	},
}

// reviewDetail returns the App Review detail of v; ok is false when v has none.
func (s *Service) reviewDetail(ctx context.Context, v Version) (asc.AppStoreReviewDetail, bool, error) {
	resp, err := s.client.AppStoreVersionsAppStoreReviewDetailGetToOneRelated(ctx, asc.AppStoreVersionsAppStoreReviewDetailGetToOneRelatedParams{ID: v.ID})
	if err != nil {
		return asc.AppStoreReviewDetail{}, false, err
	}
	return resp.Data, resp.Data.ID != "", nil
}

// ReviewTexts returns the App Review texts of v keyed by Field.API; fields
// the API has no value for are absent. It returns nil when v has no review
// detail yet (so metadata.DiffTexts reports creates).
func (s *Service) ReviewTexts(ctx context.Context, v Version) (metadata.Texts, error) {
	d, ok, err := s.reviewDetail(ctx, v)
	if err != nil || !ok {
		return nil, err
	}
	return reviewTexts(d), nil
}

func reviewTexts(d asc.AppStoreReviewDetail) metadata.Texts {
	t := metadata.Texts{}
	if d.Attributes == nil {
		return t
	}
	for api, f := range reviewFields {
		if val, ok := f.read(d.Attributes); ok {
			t[api] = val
		}
	}
	return t
}

// ApplyReviewChanges sends the changes: a POST (relationship appStoreVersion)
// when v has no review detail, else a PATCH, both with only the changed
// attributes. A demoAccountRequired change to "" means "no change" and is
// skipped. It returns the changes applied, secrets masked.
func (s *Service) ApplyReviewChanges(ctx context.Context, v Version, changes []metadata.Change) ([]metadata.Change, error) {
	d, exists, err := s.reviewDetail(ctx, v)
	if err != nil {
		return []metadata.Change{}, err
	}
	applied, _, err := s.applyReviewChanges(ctx, v, d, exists, changes)
	return applied, err
}

// applyReviewChanges is ApplyReviewChanges with the review detail d of v
// already read (exists is false when v has none). It also returns the ID of
// the detail afterwards, "" when there still is none.
func (s *Service) applyReviewChanges(ctx context.Context, v Version, d asc.AppStoreReviewDetail, exists bool, changes []metadata.Change) ([]metadata.Change, string, error) {
	applied := []metadata.Change{}
	var attrs asc.AppStoreReviewDetailUpdateRequestDataAttributes
	id := ""
	if exists {
		id = d.ID
	}
	for _, c := range changes {
		f, ok := reviewFields[c.Field]
		if c.Scope != scopeReview || !ok {
			return applied, id, clierr.New("cannot apply %s change %q to the review information", c.Scope, c.Field)
		}
		if reviewNoOp(c.Field, c.New, exists) {
			continue
		}
		f.write(&attrs, c.New)
		applied = append(applied, c)
	}
	if len(applied) == 0 {
		return applied, id, nil
	}
	masked := metadata.MaskSecrets(metadata.AppleReviewFields, applied)
	var err error
	if !exists {
		id, err = s.createReviewDetail(ctx, v, &attrs)
	} else {
		_, err = s.client.AppStoreReviewDetailsUpdateInstance(ctx, asc.AppStoreReviewDetailsUpdateInstanceParams{ID: d.ID}, asc.AppStoreReviewDetailUpdateRequest{
			Data: asc.AppStoreReviewDetailUpdateRequestData{Type: asc.ResourceTypeAppStoreReviewDetails, ID: d.ID, Attributes: &attrs},
		})
	}
	if err != nil {
		return []metadata.Change{}, id, fmt.Errorf("set review information: %w", err)
	}
	return masked, id, nil
}

// createReviewDetail creates the review detail of v with attrs (nil: empty).
func (s *Service) createReviewDetail(ctx context.Context, v Version, attrs *asc.AppStoreReviewDetailUpdateRequestDataAttributes) (string, error) {
	body := asc.AppStoreReviewDetailCreateRequest{Data: asc.AppStoreReviewDetailCreateRequestData{
		Type: asc.ResourceTypeAppStoreReviewDetails,
		Relationships: asc.AppStoreReviewDetailCreateRequestDataRelationships{
			AppStoreVersion: asc.AppStoreReviewDetailCreateRequestDataRelationshipsAppStoreVersion{
				Data: asc.ResourceIdentifier{Type: asc.ResourceTypeAppStoreVersions, ID: v.ID},
			},
		},
	}}
	if attrs != nil {
		c := asc.AppStoreReviewDetailCreateRequestDataAttributes(*attrs)
		body.Data.Attributes = &c
	}
	resp, err := s.client.AppStoreReviewDetailsCreateInstance(ctx, body)
	if err != nil {
		return "", err
	}
	return resp.Data.ID, nil
}

func (s *Service) reviewAttachments(ctx context.Context, detailID string) ([]asc.AppStoreReviewAttachment, error) {
	first, err := s.client.AppStoreReviewDetailsAppStoreReviewAttachmentsGetToManyRelated(ctx, asc.AppStoreReviewDetailsAppStoreReviewAttachmentsGetToManyRelatedParams{ID: detailID, Limit: 200})
	if err != nil {
		return nil, err
	}
	var out []asc.AppStoreReviewAttachment
	err = ascclient.Paginate(ctx, s.client, first, func(p *asc.AppStoreReviewAttachmentsResponse) *string {
		out = append(out, p.Data...)
		return p.Links.Next
	})
	return out, err
}

// ReplaceReviewAttachments deletes all attachments of v and uploads paths.
// The review detail is created when v has none.
func (s *Service) ReplaceReviewAttachments(ctx context.Context, v Version, paths []string) error {
	d, exists, err := s.reviewDetail(ctx, v)
	if err != nil {
		return err
	}
	id := ""
	if exists {
		id = d.ID
	}
	return s.replaceReviewAttachments(ctx, v, id, paths)
}

// replaceReviewAttachments is ReplaceReviewAttachments with the ID of the
// review detail of v already known, "" when v has none.
func (s *Service) replaceReviewAttachments(ctx context.Context, v Version, id string, paths []string) error {
	var err error
	if id == "" {
		if id, err = s.createReviewDetail(ctx, v, nil); err != nil {
			return fmt.Errorf("create review information: %w", err)
		}
	} else {
		old, err := s.reviewAttachments(ctx, id)
		if err != nil {
			return err
		}
		for _, a := range old {
			if err := s.client.AppStoreReviewAttachmentsDeleteInstance(ctx, asc.AppStoreReviewAttachmentsDeleteInstanceParams{ID: a.ID}); err != nil {
				return fmt.Errorf("delete attachment %s: %w", a.ID, err)
			}
		}
	}
	for _, p := range paths {
		if _, err := s.uploadAsset(ctx, attachmentKind, id, p, defaultAssetTimeout); err != nil {
			return err
		}
	}
	return nil
}

// ShowReviewInfo returns the App Review information of v. The demo password is
// masked unless reveal. A version without review information yields zero values.
func (s *Service) ShowReviewInfo(ctx context.Context, v Version, reveal bool) (ReviewInfo, error) {
	d, ok, err := s.reviewDetail(ctx, v)
	if err != nil {
		return ReviewInfo{}, err
	}
	info := ReviewInfo{Attachments: []string{}}
	if !ok {
		return info, nil
	}
	atts, err := s.reviewAttachments(ctx, d.ID)
	if err != nil {
		return ReviewInfo{}, err
	}
	info.Attachments = attachmentNames(atts)
	fillReviewInfo(&info, reviewTexts(d), reveal)
	return info, nil
}

func attachmentNames(atts []asc.AppStoreReviewAttachment) []string {
	names := []string{}
	for _, a := range atts {
		name := ""
		if a.Attributes != nil {
			name = deref(a.Attributes.FileName)
		}
		if name == "" {
			name = a.ID // still an attachment, even without a known name
		}
		names = append(names, name)
	}
	return names
}

// fillReviewInfo sets info from the review texts t: each review field goes
// to the ReviewInfo field whose json name is its flag in camel case
// (first-name -> firstName), so metadata.AppleReviewFields stays the one
// table. The demo password is masked unless reveal.
func fillReviewInfo(info *ReviewInfo, t metadata.Texts, reveal bool) {
	v := reflect.ValueOf(info).Elem()
	for _, f := range metadata.AppleReviewFields {
		val, ok := t[f.API]
		if !ok {
			continue
		}
		switch fv := jsonFieldOf(v, camel(f.Flag)); fv.Kind() {
		case reflect.String:
			fv.SetString(val)
		case reflect.Bool:
			fv.SetBool(val == "true")
		}
	}
	if !reveal && info.DemoPassword != "" {
		info.DemoPassword = metadata.Mask
	}
}

// jsonFieldOf returns the field of struct v with the json name, else the
// zero Value.
func jsonFieldOf(v reflect.Value, name string) reflect.Value {
	for i := range v.NumField() {
		if n, _, _ := strings.Cut(v.Type().Field(i).Tag.Get("json"), ","); n == name {
			return v.Field(i)
		}
	}
	return reflect.Value{}
}

// camel turns a flag name into camel case: "first-name" -> "firstName".
func camel(flag string) string {
	parts := strings.Split(flag, "-")
	for i := 1; i < len(parts); i++ {
		if parts[i] != "" {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return strings.Join(parts, "")
}

// reviewNoOp reports whether setting the review field api to val changes
// nothing: an empty demo_required.txt cannot clear the flag, and a version
// without review information (exists false) has nothing to clear.
func reviewNoOp(api, val string, exists bool) bool {
	return val == "" && (api == apiDemoRequired || !exists)
}

// SetReviewInfo changes the given review texts of v and, when attachments is
// not empty, replaces all attachments with those files. Texts and files are
// checked before any API call. With dryRun nothing is sent. It returns the
// review information after the change (password masked) and the changes
// (passwords masked).
func (s *Service) SetReviewInfo(ctx context.Context, v Version, texts metadata.Texts, attachments []string, dryRun bool) (ReviewInfo, []metadata.Change, error) {
	if err := metadata.ValidateTexts(metadata.AppleReviewFields, texts); err != nil {
		return ReviewInfo{}, nil, err
	}
	for _, p := range attachments {
		fi, err := os.Stat(p)
		if err != nil || fi.IsDir() {
			return ReviewInfo{}, nil, clierr.New("attachment %s is not a readable file", p)
		}
	}
	d, exists, err := s.reviewDetail(ctx, v)
	if err != nil {
		return ReviewInfo{}, nil, err
	}
	texts = maps.Clone(texts)
	maps.DeleteFunc(texts, func(api, val string) bool { return reviewNoOp(api, val, exists) })
	var remote metadata.Texts
	if exists {
		remote = reviewTexts(d)
	}
	changes := metadata.DiffTexts(metadata.LevelReview, "", metadata.AppleReviewFields, texts, remote)
	after := metadata.Texts{}
	for k, val := range remote {
		after[k] = val
	}
	for _, c := range changes {
		after[c.Field] = c.New
	}
	if len(attachments) > 0 {
		names := make([]string, len(attachments))
		for i, p := range attachments {
			names[i] = filepath.Base(p)
		}
		changes = append(changes, metadata.Change{Action: metadata.ActionReplace, Scope: scopeReview, Field: fieldAttachments, New: strings.Join(names, ", ")})
	}
	if !dryRun {
		textChanges := slices.DeleteFunc(slices.Clone(changes), func(c metadata.Change) bool { return c.Field == fieldAttachments })
		_, id, err := s.applyReviewChanges(ctx, v, d, exists, textChanges)
		if err != nil {
			return ReviewInfo{}, nil, err
		}
		if len(attachments) > 0 {
			if err := s.replaceReviewAttachments(ctx, v, id, attachments); err != nil {
				return ReviewInfo{}, nil, err
			}
		}
	}
	info := ReviewInfo{Attachments: []string{}}
	fillReviewInfo(&info, after, false)
	if len(attachments) > 0 {
		for _, p := range attachments {
			info.Attachments = append(info.Attachments, filepath.Base(p))
		}
	} else if exists {
		atts, err := s.reviewAttachments(ctx, d.ID)
		if err != nil {
			return ReviewInfo{}, nil, err
		}
		info.Attachments = attachmentNames(atts)
	}
	return info, metadata.MaskSecrets(metadata.AppleReviewFields, changes), nil
}

// attachmentKind is the assetKind of App Review attachments.
var attachmentKind = assetKind{
	name: "attachment",
	reserve: func(ctx context.Context, c *ascclient.Client, detailID, fileName string, size int64) (assetReservation, error) {
		resp, err := c.AppStoreReviewAttachmentsCreateInstance(ctx, asc.AppStoreReviewAttachmentCreateRequest{
			Data: asc.AppStoreReviewAttachmentCreateRequestData{
				Type:       asc.ResourceTypeAppStoreReviewAttachments,
				Attributes: asc.AppStoreReviewAttachmentCreateRequestDataAttributes{FileName: fileName, FileSize: int(size)},
				Relationships: asc.AppStoreReviewAttachmentCreateRequestDataRelationships{
					AppStoreReviewDetail: asc.AppStoreReviewAttachmentCreateRequestDataRelationshipsAppStoreReviewDetail{
						Data: asc.ResourceIdentifier{Type: asc.ResourceTypeAppStoreReviewDetails, ID: detailID},
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
		_, err := c.AppStoreReviewAttachmentsUpdateInstance(ctx, asc.AppStoreReviewAttachmentsUpdateInstanceParams{ID: id}, asc.AppStoreReviewAttachmentUpdateRequest{
			Data: asc.AppStoreReviewAttachmentUpdateRequestData{
				Type: asc.ResourceTypeAppStoreReviewAttachments,
				ID:   id,
				Attributes: &asc.AppStoreReviewAttachmentUpdateRequestDataAttributes{
					Uploaded:           asc.Value(true),
					SourceFileChecksum: asc.Value(md5),
				},
			},
		})
		return err
	},
	state: func(ctx context.Context, c *ascclient.Client, id string) (assetState, error) {
		resp, err := c.AppStoreReviewAttachmentsGetInstance(ctx, asc.AppStoreReviewAttachmentsGetInstanceParams{ID: id})
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
