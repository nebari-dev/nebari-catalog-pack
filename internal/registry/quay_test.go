package registry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestSortVersionsDesc(t *testing.T) {
	got := sortVersionsDesc([]string{"0.1.0", "0.1.0-alpha.4", "0.2.0", "latest", "0.1.0-alpha.10"})
	want := []string{"0.2.0", "0.1.0", "0.1.0-alpha.10", "0.1.0-alpha.4", "latest"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestListFiltersChartsAndResolvesTags(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v1/repository"):
			if r.URL.Query().Get("namespace") != "nebari" {
				t.Errorf("unexpected namespace query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"repositories":[
				{"name":"charts/nebari-lgtm-pack","is_public":true},
				{"name":"charts/nebari-data-science-pack","is_public":true},
				{"name":"nebari-operator","is_public":true},
				{"name":"keycloak","is_public":true}
			],"next_page":null}`))
		case strings.HasSuffix(r.URL.Path, "/nebari-lgtm-pack/tags/list"):
			_, _ = w.Write([]byte(`{"name":"nebari/charts/nebari-lgtm-pack","tags":["0.1.3","0.1.2"]}`))
		case strings.HasSuffix(r.URL.Path, "/nebari-data-science-pack/tags/list"):
			_, _ = w.Write([]byte(`{"name":"x","tags":["0.1.0"]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := New(Options{
		APIBase:   srv.URL + "/api/v1",
		OCIBase:   srv.URL + "/v2",
		Namespace: "nebari",
	})
	packs, err := c.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(packs) != 2 {
		t.Fatalf("expected 2 chart packs, got %d: %+v", len(packs), packs)
	}
	// Sorted by title: data-science before lgtm.
	if packs[0].Name != "nebari-data-science-pack" || packs[1].Name != "nebari-lgtm-pack" {
		t.Fatalf("unexpected order: %s, %s", packs[0].Name, packs[1].Name)
	}
	lgtm := packs[1]
	if lgtm.Latest != "0.1.3" {
		t.Errorf("expected latest 0.1.3, got %s", lgtm.Latest)
	}
	if !strings.HasSuffix(lgtm.OCIRef, "/nebari/charts/nebari-lgtm-pack") {
		t.Errorf("unexpected OCIRef: %s", lgtm.OCIRef)
	}
}
