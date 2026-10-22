package apple

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/nikeee/store-cli/internal/apple/asc"
	"github.com/nikeee/store-cli/internal/apple/ascclient"
	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/metadata"
)

// Localization is the listing of one locale, both levels merged.
type Localization struct {
	Locale            string `json:"locale"`
	Name              string `json:"name"`
	Subtitle          string `json:"subtitle"`
	PrivacyURL        string `json:"privacyUrl"`
	PrivacyChoicesURL string `json:"privacyChoicesUrl"`
	Description       string `json:"description"`
	Keywords          string `json:"keywords"`
	PromotionalText   string `json:"promotionalText"`
	MarketingURL      string `json:"marketingUrl"`
	SupportURL        string `json:"supportUrl"`
	WhatsNew          string `json:"whatsNew"`
}

// LocaleResult is the result of listing locale add and remove.
type LocaleResult struct {
	Locale string          `json:"locale"`
	Action metadata.Action `json:"action"`
}

// The tables below map metadata.Field.API names to the generated attribute
// structs. They are the only place that knows the generated field names; read
// (ListingTexts) and write (ApplyTextChanges) both use them.

// infoField selects one attribute of an app info localization. create is nil
// for the attribute that the create request requires as a plain string
// (createRequired).
type infoField struct {
	read           func(*asc.AppInfoLocalizationAttributes) *string
	update         func(*asc.AppInfoLocalizationUpdateRequestDataAttributes) *asc.Nullable[string]
	create         func(*asc.AppInfoLocalizationCreateRequestDataAttributes) *asc.Nullable[string]
	createRequired func(*asc.AppInfoLocalizationCreateRequestDataAttributes) *string
}

var infoFields = map[string]infoField{
	"name": {
		read:           func(a *asc.AppInfoLocalizationAttributes) *string { return a.Name },
		update:         func(a *asc.AppInfoLocalizationUpdateRequestDataAttributes) *asc.Nullable[string] { return &a.Name },
		createRequired: func(a *asc.AppInfoLocalizationCreateRequestDataAttributes) *string { return &a.Name },
	},
	"subtitle": {
		read:   func(a *asc.AppInfoLocalizationAttributes) *string { return a.Subtitle },
		update: func(a *asc.AppInfoLocalizationUpdateRequestDataAttributes) *asc.Nullable[string] { return &a.Subtitle },
		create: func(a *asc.AppInfoLocalizationCreateRequestDataAttributes) *asc.Nullable[string] { return &a.Subtitle },
	},
	"privacyPolicyUrl": {
		read: func(a *asc.AppInfoLocalizationAttributes) *string { return a.PrivacyPolicyURL },
		update: func(a *asc.AppInfoLocalizationUpdateRequestDataAttributes) *asc.Nullable[string] {
			return &a.PrivacyPolicyURL
		},
		create: func(a *asc.AppInfoLocalizationCreateRequestDataAttributes) *asc.Nullable[string] {
			return &a.PrivacyPolicyURL
		},
	},
	"privacyChoicesUrl": {
		read: func(a *asc.AppInfoLocalizationAttributes) *string { return a.PrivacyChoicesURL },
		update: func(a *asc.AppInfoLocalizationUpdateRequestDataAttributes) *asc.Nullable[string] {
			return &a.PrivacyChoicesURL
		},
		create: func(a *asc.AppInfoLocalizationCreateRequestDataAttributes) *asc.Nullable[string] {
			return &a.PrivacyChoicesURL
		},
	},
}

// versionField selects one attribute of an app store version localization.
type versionField struct {
	read   func(*asc.AppStoreVersionLocalizationAttributes) *string
	update func(*asc.AppStoreVersionLocalizationUpdateRequestDataAttributes) *asc.Nullable[string]
	create func(*asc.AppStoreVersionLocalizationCreateRequestDataAttributes) *asc.Nullable[string]
}

var versionFields = map[string]versionField{
	"description": {
		read: func(a *asc.AppStoreVersionLocalizationAttributes) *string { return a.Description },
		update: func(a *asc.AppStoreVersionLocalizationUpdateRequestDataAttributes) *asc.Nullable[string] {
			return &a.Description
		},
		create: func(a *asc.AppStoreVersionLocalizationCreateRequestDataAttributes) *asc.Nullable[string] {
			return &a.Description
		},
	},
	"keywords": {
		read: func(a *asc.AppStoreVersionLocalizationAttributes) *string { return a.Keywords },
		update: func(a *asc.AppStoreVersionLocalizationUpdateRequestDataAttributes) *asc.Nullable[string] {
			return &a.Keywords
		},
		create: func(a *asc.AppStoreVersionLocalizationCreateRequestDataAttributes) *asc.Nullable[string] {
			return &a.Keywords
		},
	},
	"promotionalText": {
		read: func(a *asc.AppStoreVersionLocalizationAttributes) *string { return a.PromotionalText },
		update: func(a *asc.AppStoreVersionLocalizationUpdateRequestDataAttributes) *asc.Nullable[string] {
			return &a.PromotionalText
		},
		create: func(a *asc.AppStoreVersionLocalizationCreateRequestDataAttributes) *asc.Nullable[string] {
			return &a.PromotionalText
		},
	},
	"marketingUrl": {
		read: func(a *asc.AppStoreVersionLocalizationAttributes) *string { return a.MarketingURL },
		update: func(a *asc.AppStoreVersionLocalizationUpdateRequestDataAttributes) *asc.Nullable[string] {
			return &a.MarketingURL
		},
		create: func(a *asc.AppStoreVersionLocalizationCreateRequestDataAttributes) *asc.Nullable[string] {
			return &a.MarketingURL
		},
	},
	"supportUrl": {
		read: func(a *asc.AppStoreVersionLocalizationAttributes) *string { return a.SupportURL },
		update: func(a *asc.AppStoreVersionLocalizationUpdateRequestDataAttributes) *asc.Nullable[string] {
			return &a.SupportURL
		},
		create: func(a *asc.AppStoreVersionLocalizationCreateRequestDataAttributes) *asc.Nullable[string] {
			return &a.SupportURL
		},
	},
	"whatsNew": {
		read: func(a *asc.AppStoreVersionLocalizationAttributes) *string { return a.WhatsNew },
		update: func(a *asc.AppStoreVersionLocalizationUpdateRequestDataAttributes) *asc.Nullable[string] {
			return &a.WhatsNew
		},
		create: func(a *asc.AppStoreVersionLocalizationCreateRequestDataAttributes) *asc.Nullable[string] {
			return &a.WhatsNew
		},
	},
}

// localizationFields selects the Localization field of an API name.
var localizationFields = map[string]func(*Localization) *string{
	"name":              func(l *Localization) *string { return &l.Name },
	"subtitle":          func(l *Localization) *string { return &l.Subtitle },
	"privacyPolicyUrl":  func(l *Localization) *string { return &l.PrivacyURL },
	"privacyChoicesUrl": func(l *Localization) *string { return &l.PrivacyChoicesURL },
	"description":       func(l *Localization) *string { return &l.Description },
	"keywords":          func(l *Localization) *string { return &l.Keywords },
	"promotionalText":   func(l *Localization) *string { return &l.PromotionalText },
	"marketingUrl":      func(l *Localization) *string { return &l.MarketingURL },
	"supportUrl":        func(l *Localization) *string { return &l.SupportURL },
	"whatsNew":          func(l *Localization) *string { return &l.WhatsNew },
}

// nullable sends v, or null to clear the field when v is empty.
func nullable(v string) asc.Nullable[string] {
	if v == "" {
		return asc.Null[string]()
	}
	return asc.Value(v)
}

// editableAppInfoStates are the app info states in which texts can change.
var editableAppInfoStates = []asc.AppInfoAttributesState{
	asc.AppInfoAttributesStatePrepareForSubmission,
	asc.AppInfoAttributesStateDeveloperRejected,
	asc.AppInfoAttributesStateRejected,
}

func (s *Service) appInfos(ctx context.Context, app App) ([]asc.AppInfo, error) {
	resp, err := s.client.AppsAppInfosGetToManyRelated(ctx, asc.AppsAppInfosGetToManyRelatedParams{ID: app.ID})
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func appInfoState(i asc.AppInfo) asc.AppInfoAttributesState {
	if i.Attributes == nil {
		return ""
	}
	return deref(i.Attributes.State)
}

// EditableAppInfo returns the ID of the app info that can still be edited.
func (s *Service) EditableAppInfo(ctx context.Context, app App) (string, error) {
	infos, err := s.appInfos(ctx, app)
	if err != nil {
		return "", err
	}
	for _, i := range infos {
		if slices.Contains(editableAppInfoStates, appInfoState(i)) {
			return i.ID, nil
		}
	}
	e := clierr.New("app metadata is locked (in review); name, subtitle and privacy URLs cannot change now")
	e.Code = "STATE_ERROR"
	return "", e
}

// readableAppInfo returns the editable app info, else the live one.
func (s *Service) readableAppInfo(ctx context.Context, app App) (string, bool, error) {
	infos, err := s.appInfos(ctx, app)
	if err != nil {
		return "", false, err
	}
	for _, i := range infos {
		if slices.Contains(editableAppInfoStates, appInfoState(i)) {
			return i.ID, true, nil
		}
	}
	for _, i := range infos {
		if appInfoState(i) == asc.AppInfoAttributesStateReadyForDistribution {
			return i.ID, true, nil
		}
	}
	return "", false, nil
}

func (s *Service) infoLocalizations(ctx context.Context, infoID string) ([]asc.AppInfoLocalization, error) {
	first, err := s.client.AppInfosAppInfoLocalizationsGetToManyRelated(ctx, asc.AppInfosAppInfoLocalizationsGetToManyRelatedParams{ID: infoID, Limit: 200})
	if err != nil {
		return nil, err
	}
	var out []asc.AppInfoLocalization
	err = ascclient.Paginate(ctx, s.client, first, func(p *asc.AppInfoLocalizationsResponse) *string {
		out = append(out, p.Data...)
		return p.Links.Next
	})
	return out, err
}

func (s *Service) versionLocalizations(ctx context.Context, versionID string) ([]asc.AppStoreVersionLocalization, error) {
	first, err := s.client.AppStoreVersionsAppStoreVersionLocalizationsGetToManyRelated(ctx, asc.AppStoreVersionsAppStoreVersionLocalizationsGetToManyRelatedParams{ID: versionID, Limit: 200})
	if err != nil {
		return nil, err
	}
	var out []asc.AppStoreVersionLocalization
	err = ascclient.Paginate(ctx, s.client, first, func(p *asc.AppStoreVersionLocalizationsResponse) *string {
		out = append(out, p.Data...)
		return p.Links.Next
	})
	return out, err
}

// ListingTexts returns, per locale, the texts of the app info and the version
// level, keyed by Field.API. Fields the API has no value for are absent.
func (s *Service) ListingTexts(ctx context.Context, app App, v Version) (map[string]metadata.Texts, error) {
	out := map[string]metadata.Texts{}
	texts := func(locale string) metadata.Texts {
		if out[locale] == nil {
			out[locale] = metadata.Texts{}
		}
		return out[locale]
	}
	infoID, ok, err := s.readableAppInfo(ctx, app)
	if err != nil {
		return nil, err
	}
	if ok {
		locs, err := s.infoLocalizations(ctx, infoID)
		if err != nil {
			return nil, err
		}
		for _, l := range locs {
			if l.Attributes == nil || l.Attributes.Locale == nil {
				continue
			}
			t := texts(*l.Attributes.Locale)
			for api, f := range infoFields {
				if p := f.read(l.Attributes); p != nil {
					t[api] = *p
				}
			}
		}
	}
	locs, err := s.versionLocalizations(ctx, v.ID)
	if err != nil {
		return nil, err
	}
	for _, l := range locs {
		if l.Attributes == nil || l.Attributes.Locale == nil {
			continue
		}
		t := texts(*l.Attributes.Locale)
		for api, f := range versionFields {
			if p := f.read(l.Attributes); p != nil {
				t[api] = *p
			}
		}
	}
	return out, nil
}

func localizationFromTexts(locale string, t metadata.Texts) Localization {
	l := Localization{Locale: locale}
	for _, f := range metadata.AppleFields {
		if sel, ok := localizationFields[f.API]; ok {
			if v, ok := t[f.API]; ok {
				*sel(&l) = v
			}
		}
	}
	return l
}

// ShowListing returns the localizations of v sorted by locale, only locale
// when it is not empty (a missing one is an error).
func (s *Service) ShowListing(ctx context.Context, app App, v Version, locale string) ([]Localization, error) {
	all, err := s.ListingTexts(ctx, app, v)
	if err != nil {
		return nil, err
	}
	out := []Localization{}
	for loc, t := range all {
		if locale == "" || loc == locale {
			out = append(out, localizationFromTexts(loc, t))
		}
	}
	if locale != "" && len(out) == 0 {
		e := clierr.New("locale %s not found", locale)
		e.Hint = "store apple listing locale add " + locale
		return nil, e
	}
	slices.SortFunc(out, func(a, b Localization) int { return strings.Compare(a.Locale, b.Locale) })
	return out, nil
}

// SetListing changes the given texts of locale on v: it validates them,
// diffs against the remote state and applies the differences. The locale is
// created at the levels touched. With dryRun nothing is sent. It returns the
// listing after the change and the changes.
func (s *Service) SetListing(ctx context.Context, app App, v Version, locale string, texts metadata.Texts, dryRun bool) (Localization, []metadata.Change, error) {
	if err := metadata.ValidateLocaleTexts(locale, metadata.AppleFields, texts); err != nil {
		return Localization{}, nil, err
	}
	all, err := s.ListingTexts(ctx, app, v)
	if err != nil {
		return Localization{}, nil, err
	}
	remote := all[locale]
	changes := metadata.DiffTexts(metadata.LevelAppInfo, locale, metadata.AppleFields, texts, remote)
	changes = append(changes, metadata.DiffTexts(metadata.LevelVersion, locale, metadata.AppleFields, texts, remote)...)
	after := metadata.Texts{}
	for k, val := range remote {
		after[k] = val
	}
	for _, c := range changes {
		after[c.Field] = c.New
	}
	loc := localizationFromTexts(locale, after)
	if slices.ContainsFunc(changes, func(c metadata.Change) bool { return c.Scope == string(metadata.LevelAppInfo) }) {
		if _, err := s.EditableAppInfo(ctx, app); err != nil {
			return Localization{}, nil, err
		}
	}
	if dryRun || len(changes) == 0 {
		return loc, changes, nil
	}
	applied, err := s.ApplyTextChanges(ctx, app, v, changes)
	if err != nil {
		return Localization{}, applied, err
	}
	return loc, applied, nil
}

// changeGroup is the changes of one scope and locale.
type changeGroup struct {
	scope, locale string
	changes       []metadata.Change
}

func groupChanges(changes []metadata.Change) []*changeGroup {
	var groups []*changeGroup
	for _, c := range changes {
		i := slices.IndexFunc(groups, func(g *changeGroup) bool { return g.scope == c.Scope && g.locale == c.Locale })
		if i < 0 {
			groups = append(groups, &changeGroup{scope: c.Scope, locale: c.Locale})
			i = len(groups) - 1
		}
		groups[i].changes = append(groups[i].changes, c)
	}
	return groups
}

// ApplyTextChanges sends the app info and version level changes, one POST or
// PATCH per scope and locale with only the changed attributes. It returns the
// changes applied so far, also on error. App info changes need an editable
// app info.
func (s *Service) ApplyTextChanges(ctx context.Context, app App, v Version, changes []metadata.Change) ([]metadata.Change, error) {
	applied := []metadata.Change{}
	var (
		infoID   string
		infoLocs map[string]string
		verLocs  map[string]string
	)
	for _, g := range groupChanges(changes) {
		var err error
		switch g.scope {
		case string(metadata.LevelAppInfo):
			if infoLocs == nil {
				if infoID, err = s.EditableAppInfo(ctx, app); err != nil {
					return applied, err
				}
				if infoLocs, err = s.infoLocalizationIDs(ctx, infoID); err != nil {
					return applied, err
				}
			}
			err = s.applyInfoGroup(ctx, app, infoID, infoLocs[g.locale], g)
		case string(metadata.LevelVersion):
			if verLocs == nil {
				if verLocs, err = s.versionLocalizationIDs(ctx, v.ID); err != nil {
					return applied, err
				}
			}
			err = s.applyVersionGroup(ctx, v.ID, verLocs[g.locale], g)
		default:
			err = clierr.New("cannot apply %s changes to the listing", g.scope)
		}
		if err != nil {
			return applied, fmt.Errorf("set %s texts of %s: %w", g.scope, g.locale, err)
		}
		applied = append(applied, g.changes...)
	}
	return applied, nil
}

func (s *Service) infoLocalizationIDs(ctx context.Context, infoID string) (map[string]string, error) {
	locs, err := s.infoLocalizations(ctx, infoID)
	if err != nil {
		return nil, err
	}
	ids := map[string]string{}
	for _, l := range locs {
		if l.Attributes != nil && l.Attributes.Locale != nil {
			ids[*l.Attributes.Locale] = l.ID
		}
	}
	return ids, nil
}

func (s *Service) versionLocalizationIDs(ctx context.Context, versionID string) (map[string]string, error) {
	locs, err := s.versionLocalizations(ctx, versionID)
	if err != nil {
		return nil, err
	}
	ids := map[string]string{}
	for _, l := range locs {
		if l.Attributes != nil && l.Attributes.Locale != nil {
			ids[*l.Attributes.Locale] = l.ID
		}
	}
	return ids, nil
}

// applyInfoGroup updates the app info localization id, or creates it when id
// is empty. A created one is named like the app unless the group sets a name.
func (s *Service) applyInfoGroup(ctx context.Context, app App, infoID, id string, g *changeGroup) error {
	if id != "" {
		attrs := asc.AppInfoLocalizationUpdateRequestDataAttributes{}
		for _, c := range g.changes {
			*infoFields[c.Field].update(&attrs) = nullable(c.New)
		}
		_, err := s.client.AppInfoLocalizationsUpdateInstance(ctx, asc.AppInfoLocalizationsUpdateInstanceParams{ID: id},
			asc.AppInfoLocalizationUpdateRequest{Data: asc.AppInfoLocalizationUpdateRequestData{Type: asc.ResourceTypeAppInfoLocalizations, ID: id, Attributes: &attrs}})
		return err
	}
	return s.createInfoLocalization(ctx, app, infoID, g.locale, g.changes)
}

func (s *Service) createInfoLocalization(ctx context.Context, app App, infoID, locale string, changes []metadata.Change) error {
	attrs := asc.AppInfoLocalizationCreateRequestDataAttributes{Locale: locale, Name: app.Name}
	for _, c := range changes {
		f := infoFields[c.Field]
		switch {
		case f.createRequired != nil:
			if c.New != "" {
				*f.createRequired(&attrs) = c.New
			}
		case c.New != "":
			*f.create(&attrs) = asc.Value(c.New)
		}
	}
	_, err := s.client.AppInfoLocalizationsCreateInstance(ctx, asc.AppInfoLocalizationCreateRequest{Data: asc.AppInfoLocalizationCreateRequestData{
		Type:       asc.ResourceTypeAppInfoLocalizations,
		Attributes: attrs,
		Relationships: asc.AppInfoLocalizationCreateRequestDataRelationships{
			AppInfo: asc.AppInfoLocalizationCreateRequestDataRelationshipsAppInfo{Data: asc.ResourceIdentifier{Type: asc.ResourceTypeAppInfos, ID: infoID}},
		},
	}})
	return err
}

func (s *Service) applyVersionGroup(ctx context.Context, versionID, id string, g *changeGroup) error {
	if id != "" {
		attrs := asc.AppStoreVersionLocalizationUpdateRequestDataAttributes{}
		for _, c := range g.changes {
			*versionFields[c.Field].update(&attrs) = nullable(c.New)
		}
		_, err := s.client.AppStoreVersionLocalizationsUpdateInstance(ctx, asc.AppStoreVersionLocalizationsUpdateInstanceParams{ID: id},
			asc.AppStoreVersionLocalizationUpdateRequest{Data: asc.AppStoreVersionLocalizationUpdateRequestData{Type: asc.ResourceTypeAppStoreVersionLocalizations, ID: id, Attributes: &attrs}})
		return err
	}
	return s.createVersionLocalization(ctx, versionID, g.locale, g.changes)
}

func (s *Service) createVersionLocalization(ctx context.Context, versionID, locale string, changes []metadata.Change) error {
	attrs := asc.AppStoreVersionLocalizationCreateRequestDataAttributes{Locale: locale}
	for _, c := range changes {
		if c.New != "" {
			*versionFields[c.Field].create(&attrs) = asc.Value(c.New)
		}
	}
	_, err := s.client.AppStoreVersionLocalizationsCreateInstance(ctx, asc.AppStoreVersionLocalizationCreateRequest{Data: asc.AppStoreVersionLocalizationCreateRequestData{
		Type:       asc.ResourceTypeAppStoreVersionLocalizations,
		Attributes: attrs,
		Relationships: asc.AppStoreVersionLocalizationCreateRequestDataRelationships{
			AppStoreVersion: asc.AppStoreVersionLocalizationCreateRequestDataRelationshipsAppStoreVersion{Data: asc.ResourceIdentifier{Type: asc.ResourceTypeAppStoreVersions, ID: versionID}},
		},
	}})
	return err
}

// AddLocale creates the localization of locale at the app info and version
// level where it is missing. Both present is a skip. With dryRun nothing is
// sent.
func (s *Service) AddLocale(ctx context.Context, app App, v Version, locale string, dryRun bool) (LocaleResult, error) {
	verLocs, err := s.versionLocalizationIDs(ctx, v.ID)
	if err != nil {
		return LocaleResult{}, err
	}
	_, haveVer := verLocs[locale]
	infoID, ok, err := s.readableAppInfo(ctx, app)
	if err != nil {
		return LocaleResult{}, err
	}
	infoLocs := map[string]string{}
	if ok {
		if infoLocs, err = s.infoLocalizationIDs(ctx, infoID); err != nil {
			return LocaleResult{}, err
		}
	}
	_, haveInfo := infoLocs[locale]
	if !haveInfo {
		if infoID, err = s.EditableAppInfo(ctx, app); err != nil {
			return LocaleResult{}, err
		}
	}
	res := LocaleResult{Locale: locale, Action: metadata.ActionSkip}
	if haveVer && haveInfo {
		return res, nil
	}
	res.Action = metadata.ActionCreate
	if dryRun {
		return res, nil
	}
	if !haveInfo {
		if err := s.createInfoLocalization(ctx, app, infoID, locale, nil); err != nil {
			return LocaleResult{}, fmt.Errorf("add app info locale %s: %w", locale, err)
		}
	}
	if !haveVer {
		if err := s.createVersionLocalization(ctx, v.ID, locale, nil); err != nil {
			return LocaleResult{}, fmt.Errorf("add version locale %s: %w", locale, err)
		}
	}
	return res, nil
}

// RemoveLocale deletes the localization of locale at both levels. The
// primary locale is refused. A locale that does not exist is a skip. With
// dryRun nothing is sent.
func (s *Service) RemoveLocale(ctx context.Context, app App, v Version, locale string, dryRun bool) (LocaleResult, error) {
	if locale == app.PrimaryLocale {
		return LocaleResult{}, clierr.New("%s is the primary locale of the app and cannot be removed", locale)
	}
	verLocs, err := s.versionLocalizationIDs(ctx, v.ID)
	if err != nil {
		return LocaleResult{}, err
	}
	infoID, ok, err := s.readableAppInfo(ctx, app)
	if err != nil {
		return LocaleResult{}, err
	}
	infoLocs := map[string]string{}
	if ok {
		if infoLocs, err = s.infoLocalizationIDs(ctx, infoID); err != nil {
			return LocaleResult{}, err
		}
	}
	verID, haveVer := verLocs[locale]
	infoLocID, haveInfo := infoLocs[locale]
	res := LocaleResult{Locale: locale, Action: metadata.ActionSkip}
	if !haveVer && !haveInfo {
		return res, nil
	}
	if haveInfo {
		if _, err := s.EditableAppInfo(ctx, app); err != nil {
			return LocaleResult{}, err
		}
	}
	res.Action = metadata.ActionDelete
	if dryRun {
		return res, nil
	}
	if haveVer {
		if err := s.client.AppStoreVersionLocalizationsDeleteInstance(ctx, asc.AppStoreVersionLocalizationsDeleteInstanceParams{ID: verID}); err != nil {
			return LocaleResult{}, fmt.Errorf("remove version locale %s: %w", locale, err)
		}
	}
	if haveInfo {
		if err := s.client.AppInfoLocalizationsDeleteInstance(ctx, asc.AppInfoLocalizationsDeleteInstanceParams{ID: infoLocID}); err != nil {
			return LocaleResult{}, fmt.Errorf("remove app info locale %s: %w", locale, err)
		}
	}
	return res, nil
}
