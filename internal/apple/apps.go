package apple

import (
	"context"
	"slices"

	"github.com/nikeee/store-cli/internal/apple/asc"
	"github.com/nikeee/store-cli/internal/apple/ascclient"
)

// shownVersions is how many versions app show lists.
const shownVersions = 5

// CheckAuth verifies the credentials with one read call.
func (s *Service) CheckAuth(ctx context.Context) error {
	_, err := s.client.AppsGetCollection(ctx, asc.AppsGetCollectionParams{Limit: 1})
	return err
}

// ResolveApp returns the app with bundleID.
func (s *Service) ResolveApp(ctx context.Context, bundleID string) (App, error) {
	a, err := s.client.ResolveApp(ctx, bundleID)
	if err != nil {
		return App{}, err
	}
	return appFromAPI(a), nil
}

// ListApps returns all apps of the team.
func (s *Service) ListApps(ctx context.Context) ([]App, error) {
	first, err := s.client.AppsGetCollection(ctx, asc.AppsGetCollectionParams{Limit: 200})
	if err != nil {
		return nil, err
	}
	apps := []App{}
	err = ascclient.Paginate(ctx, s.client, first, func(p *asc.AppsResponse) *string {
		for _, a := range p.Data {
			apps = append(apps, appFromAPI(a))
		}
		return p.Links.Next
	})
	return apps, err
}

// AppDetails is the result of app show.
type AppDetails struct {
	App      App       `json:"app"`
	Versions []Version `json:"versions"`
	Locales  []string  `json:"locales"`
}

// ShowApp returns app with its newest versions for platform (a --platform
// value) and the locales of its app info.
func (s *Service) ShowApp(ctx context.Context, app App, platform string) (AppDetails, error) {
	versions, err := s.listVersions(ctx, app, platform, "", shownVersions)
	if err != nil {
		return AppDetails{}, err
	}
	locales, err := s.appLocales(ctx, app.ID)
	if err != nil {
		return AppDetails{}, err
	}
	return AppDetails{App: app, Versions: versions, Locales: locales}, nil
}

// appLocales returns the sorted locales of the app's newest app info: the
// editable one while a version is prepared, else the live one.
func (s *Service) appLocales(ctx context.Context, appID string) ([]string, error) {
	infos, err := s.client.AppsAppInfosGetToManyRelated(ctx, asc.AppsAppInfosGetToManyRelatedParams{ID: appID})
	if err != nil {
		return nil, err
	}
	if len(infos.Data) == 0 {
		return []string{}, nil
	}
	info := infos.Data[0]
	for _, i := range infos.Data {
		if i.Attributes != nil && deref(i.Attributes.State) != asc.AppInfoAttributesStateReadyForDistribution {
			info = i
			break
		}
	}
	first, err := s.client.AppInfosAppInfoLocalizationsGetToManyRelated(ctx, asc.AppInfosAppInfoLocalizationsGetToManyRelatedParams{
		ID:                         info.ID,
		FieldsAppInfoLocalizations: []string{"locale"},
		Limit:                      200,
	})
	if err != nil {
		return nil, err
	}
	locales := []string{}
	err = ascclient.Paginate(ctx, s.client, first, func(p *asc.AppInfoLocalizationsResponse) *string {
		for _, l := range p.Data {
			if l.Attributes != nil && l.Attributes.Locale != nil {
				locales = append(locales, *l.Attributes.Locale)
			}
		}
		return p.Links.Next
	})
	slices.Sort(locales)
	return locales, err
}
