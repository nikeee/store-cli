package asc_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/nikeee/store-cli/internal/apple/asc"
)

type request struct {
	method, path, rawQuery, body string
}

// serve starts a server that answers every request with status and body and
// returns a client for it and the last request it saw.
func serve(t *testing.T, status int, body string) (*asc.Client, *request) {
	t.Helper()
	last := &request{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		*last = request{r.Method, r.URL.Path, r.URL.RawQuery, string(b)}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return asc.NewClient(srv.URL, srv.Client()), last
}

func fixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestDecodeFixtures(t *testing.T) {
	tests := []struct {
		fixture string
		status  int
		check   func(t *testing.T, c *asc.Client)
	}{
		{"builds_collection.json", http.StatusOK, func(t *testing.T, c *asc.Client) {
			resp, err := c.BuildsGetCollection(t.Context(), asc.BuildsGetCollectionParams{})
			if err != nil {
				t.Fatal(err)
			}
			a := resp.Data[0].Attributes
			if *a.Version != "42" || *a.ProcessingState != asc.BuildAttributesProcessingStateValid || a.UploadedDate.Day() != 20 {
				t.Errorf("attributes = %s %s %s", *a.Version, *a.ProcessingState, a.UploadedDate)
			}
			if resp.Links.Next == nil || !strings.Contains(*resp.Links.Next, "cursor=Mg") {
				t.Errorf("links.next = %v", resp.Links.Next)
			}
		}},
		{"appScreenshot_create.json", http.StatusCreated, func(t *testing.T, c *asc.Client) {
			resp, err := c.AppScreenshotsCreateInstance(t.Context(), asc.AppScreenshotCreateRequest{})
			if err != nil {
				t.Fatal(err)
			}
			ops := resp.Data.Attributes.UploadOperations
			if len(ops) != 2 {
				t.Fatalf("got %d upload operations", len(ops))
			}
			op := ops[1]
			h := op.RequestHeaders[0]
			if *op.Method != "PUT" || !strings.HasSuffix(*op.URL, "part-2?token=abc") || *op.Offset != 1048576 || *op.Length != 524288 ||
				*h.Name != "Content-Type" || *h.Value != "image/png" {
				t.Errorf("upload operation = %s %s %d %d %s: %s", *op.Method, *op.URL, *op.Offset, *op.Length, *h.Name, *h.Value)
			}
		}},
		{"appScreenshot_get.json", http.StatusOK, func(t *testing.T, c *asc.Client) {
			resp, err := c.AppScreenshotsGetInstance(t.Context(), asc.AppScreenshotsGetInstanceParams{ID: "s1"})
			if err != nil {
				t.Fatal(err)
			}
			st := resp.Data.Attributes.AssetDeliveryState
			if *st.State != asc.AppMediaAssetStateStateFailed || *st.Errors[0].Code != "IMAGE_INCORRECT_DIMENSIONS" || *st.Errors[0].Description == "" {
				t.Errorf("assetDeliveryState = %s %s", *st.State, *st.Errors[0].Code)
			}
		}},
		{"reviewSubmissions_with_included.json", http.StatusOK, func(t *testing.T, c *asc.Client) {
			resp, err := c.ReviewSubmissionsGetCollection(t.Context(), asc.ReviewSubmissionsGetCollectionParams{})
			if err != nil {
				t.Fatal(err)
			}
			versionID := resp.Data[0].Relationships.AppStoreVersionForReview.Data.ID
			if versionID != "a1b2c3d4-e5f6-4a7b-8c9d-0e1f2a3b4c5d" {
				t.Errorf("appStoreVersionForReview = %s", versionID)
			}
			if len(resp.Included) != 2 {
				t.Fatalf("got %d included", len(resp.Included))
			}
			var item asc.ReviewSubmissionItem
			var version asc.AppStoreVersion
			for _, inc := range resp.Included {
				switch inc.Type {
				case asc.ResourceTypeReviewSubmissionItems:
					err = inc.Decode(&item)
				case asc.ResourceTypeAppStoreVersions:
					err = inc.Decode(&version)
				default:
					t.Errorf("unexpected included type %s", inc.Type)
				}
				if err != nil {
					t.Fatal(err)
				}
			}
			if *item.Attributes.State != asc.ReviewSubmissionItemAttributesStateReadyForReview {
				t.Errorf("item state = %s", *item.Attributes.State)
			}
			if version.ID != versionID || *version.Attributes.VersionString != "2.3.0" {
				t.Errorf("version = %s %s", version.ID, *version.Attributes.VersionString)
			}
		}},
		{"error_409.json", http.StatusConflict, func(t *testing.T, c *asc.Client) {
			_, err := c.AppStoreVersionLocalizationsUpdateInstance(t.Context(), asc.AppStoreVersionLocalizationsUpdateInstanceParams{ID: "X"}, asc.AppStoreVersionLocalizationUpdateRequest{})
			var re *asc.ResponseError
			if !errors.As(err, &re) {
				t.Fatalf("err = %v, want *asc.ResponseError", err)
			}
			e := re.Response.Errors[0]
			if re.StatusCode != http.StatusConflict || e.Code != "ENTITY_ERROR.ATTRIBUTE.INVALID" || *e.Source.Pointer != "/data/attributes/whatsNew" {
				t.Errorf("error = %d %s %v", re.StatusCode, e.Code, e.Source)
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			c, _ := serve(t, tt.status, fixture(t, tt.fixture))
			tt.check(t, c)
		})
	}
}

func TestPatchOmitsUnsetAttributes(t *testing.T) {
	c, got := serve(t, http.StatusOK, `{"data":{"type":"appStoreVersionLocalizations","id":"X"},"links":{"self":"x"}}`)
	_, err := c.AppStoreVersionLocalizationsUpdateInstance(t.Context(),
		asc.AppStoreVersionLocalizationsUpdateInstanceParams{ID: "X"},
		asc.AppStoreVersionLocalizationUpdateRequest{Data: asc.AppStoreVersionLocalizationUpdateRequestData{
			Type:       asc.ResourceTypeAppStoreVersionLocalizations,
			ID:         "X",
			Attributes: &asc.AppStoreVersionLocalizationUpdateRequestDataAttributes{WhatsNew: new("Bug fixes")},
		}})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"data":{"type":"appStoreVersionLocalizations","id":"X","attributes":{"whatsNew":"Bug fixes"}}}`
	if got.method != http.MethodPatch || got.path != "/v1/appStoreVersionLocalizations/X" || got.body != want {
		t.Errorf("got %s %s %s\nwant PATCH /v1/appStoreVersionLocalizations/X %s", got.method, got.path, got.body, want)
	}
}

func TestQueryParamsEncoded(t *testing.T) {
	c, got := serve(t, http.StatusOK, `{"data":[],"links":{"self":"x"}}`)
	_, err := c.BuildsGetCollection(t.Context(), asc.BuildsGetCollectionParams{
		FilterApp:    []string{"1"},
		Limit:        200,
		FieldsBuilds: []string{"version", "uploadedDate"},
	})
	if err != nil {
		t.Fatal(err)
	}
	q := got.rawQuery
	if !strings.Contains(q, "filter%5Bapp%5D=1") && !strings.Contains(q, "filter[app]=1") {
		t.Errorf("query %q lacks filter[app]=1", q)
	}
	if !strings.Contains(q, "limit=200") {
		t.Errorf("query %q lacks limit=200", q)
	}
	if !strings.Contains(q, "fields%5Bbuilds%5D=version%2CuploadedDate") {
		t.Errorf("query %q lacks comma-joined fields[builds]", q)
	}
}

func TestGetNext(t *testing.T) {
	var paths []string
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.RequestURI())
		next := ""
		if r.URL.Query().Get("cursor") == "" {
			next = `,"next":"` + srv.URL + `/v1/builds?cursor=2"`
		}
		_, _ = io.WriteString(w, `{"data":[{"type":"builds","id":"b`+r.URL.Query().Get("cursor")+`"}],"links":{"self":"x"`+next+`}}`)
	}))
	t.Cleanup(srv.Close)
	c := asc.NewClient(srv.URL, srv.Client())

	page, err := c.BuildsGetCollection(t.Context(), asc.BuildsGetCollectionParams{})
	if err != nil {
		t.Fatal(err)
	}
	page, err = asc.GetNext[asc.BuildsResponse](t.Context(), c, *page.Links.Next)
	if err != nil {
		t.Fatal(err)
	}
	if page.Data[0].ID != "b2" || page.Links.Next != nil {
		t.Errorf("second page = %s, next %v", page.Data[0].ID, page.Links.Next)
	}

	if _, err := asc.GetNext[asc.BuildsResponse](t.Context(), c, "https://example.com/v1/builds?cursor=3"); err == nil {
		t.Error("GetNext followed a link to another host")
	}
	if want := []string{"/v1/builds", "/v1/builds?cursor=2"}; strings.Join(paths, " ") != strings.Join(want, " ") {
		t.Errorf("requests = %v, want %v", paths, want)
	}
}
