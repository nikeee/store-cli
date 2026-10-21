package ascclient_test

import (
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/nikeee/store-cli/internal/apple/asc"
	"github.com/nikeee/store-cli/internal/apple/ascclient"
	"github.com/nikeee/store-cli/internal/testutil/ascfake"
)

func TestPaginationFollowsNext(t *testing.T) {
	srv := ascfake.New(t)
	page1, page2 := ascfake.JSON(http.StatusOK, "apps_page1.json"), ascfake.JSON(http.StatusOK, "apps_page2.json")
	srv.Handle("GET", "/v1/apps", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("cursor") == "Mg" {
			page2(w, r)
			return
		}
		page1(w, r)
	})
	c := newClient(t, srv, time.Now)

	first, err := c.AppsGetCollection(t.Context(), asc.AppsGetCollectionParams{Limit: 200})
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	err = ascclient.Paginate(t.Context(), c, first, func(p *asc.AppsResponse) *string {
		for _, a := range p.Data {
			ids = append(ids, a.ID)
		}
		return p.Links.Next
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"1234567890", "1234567891", "1234567892"}; !slices.Equal(ids, want) {
		t.Errorf("ids = %v, want %v", ids, want)
	}
	if n := len(srv.Requests()); n != 2 {
		t.Errorf("%d requests, want 2", n)
	}
}
