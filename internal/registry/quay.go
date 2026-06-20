package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
)

// Client enumerates packs from a Quay-compatible registry.
type Client struct {
	apiBase     string
	ociBase     string
	namespace   string
	chartPrefix string
	ociPullBase string
	http        *http.Client
}

// Options configures a Client. Zero values fall back to quay/nebari defaults.
type Options struct {
	APIBase     string
	OCIBase     string
	Namespace   string
	ChartPrefix string
	OCIPullBase string
	HTTPClient  *http.Client
}

// New builds a registry client.
func New(o Options) *Client {
	c := &Client{
		apiBase:     orDefault(o.APIBase, "https://quay.io/api/v1"),
		ociBase:     orDefault(o.OCIBase, "https://quay.io/v2"),
		namespace:   orDefault(o.Namespace, "nebari"),
		chartPrefix: orDefault(o.ChartPrefix, "charts"),
		ociPullBase: o.OCIPullBase,
		http:        o.HTTPClient,
	}
	if c.http == nil {
		c.http = &http.Client{Timeout: 15 * time.Second}
	}
	if c.ociPullBase == "" {
		c.ociPullBase = fmt.Sprintf("oci://%s/%s/%s", hostOf(c.ociBase), c.namespace, c.chartPrefix)
	}
	return c
}

// List discovers all chart packs in the namespace and resolves their versions.
// Packs are returned sorted by display title. Version resolution per pack is
// best-effort: a pack whose tags cannot be fetched is still returned (with an
// empty Versions slice) rather than dropped.
func (c *Client) List(ctx context.Context) ([]Pack, error) {
	repos, err := c.listChartRepos(ctx)
	if err != nil {
		return nil, err
	}
	packs := make([]Pack, 0, len(repos))
	for _, name := range repos {
		p := Pack{
			Name:   name,
			OCIRef: fmt.Sprintf("%s/%s", c.ociPullBase, name),
		}
		if tags, err := c.listTags(ctx, name); err == nil {
			p.Versions = sortVersionsDesc(tags)
			if len(p.Versions) > 0 {
				p.Latest = p.Versions[0]
			}
		}
		packs = append(packs, p)
	}
	sort.Slice(packs, func(i, j int) bool {
		return strings.ToLower(packs[i].Title()) < strings.ToLower(packs[j].Title())
	})
	return packs, nil
}

// listChartRepos returns chart names (prefix stripped) under the namespace,
// following pagination.
func (c *Client) listChartRepos(ctx context.Context) ([]string, error) {
	var names []string
	page := ""
	prefix := c.chartPrefix + "/"
	for {
		u, _ := url.Parse(c.apiBase + "/repository")
		q := u.Query()
		q.Set("namespace", c.namespace)
		q.Set("public", "true")
		if page != "" {
			q.Set("next_page", page)
		}
		u.RawQuery = q.Encode()

		var out quayRepoList
		if err := c.getJSON(ctx, u.String(), &out); err != nil {
			return nil, fmt.Errorf("list repositories: %w", err)
		}
		for _, r := range out.Repositories {
			if strings.HasPrefix(r.Name, prefix) {
				names = append(names, strings.TrimPrefix(r.Name, prefix))
			}
		}
		if out.NextPage == "" || out.NextPage == page {
			break
		}
		page = out.NextPage
	}
	sort.Strings(names)
	return names, nil
}

// listTags returns the OCI tags for a chart.
func (c *Client) listTags(ctx context.Context, chart string) ([]string, error) {
	u := fmt.Sprintf("%s/%s/%s/%s/tags/list", c.ociBase, c.namespace, c.chartPrefix, chart)
	var out ociTagList
	if err := c.getJSON(ctx, u, &out); err != nil {
		return nil, err
	}
	return out.Tags, nil
}

func (c *Client) getJSON(ctx context.Context, urlStr string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: status %d", urlStr, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}

// sortVersionsDesc sorts SemVer tags newest-first. Non-SemVer tags sort after
// valid ones, lexicographically descending, so they remain visible.
func sortVersionsDesc(tags []string) []string {
	type parsed struct {
		raw string
		v   *semver.Version
	}
	items := make([]parsed, 0, len(tags))
	for _, t := range tags {
		v, err := semver.NewVersion(t)
		if err != nil {
			items = append(items, parsed{raw: t})
			continue
		}
		items = append(items, parsed{raw: t, v: v})
	}
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		switch {
		case a.v != nil && b.v != nil:
			return a.v.GreaterThan(b.v)
		case a.v != nil:
			return true // valid SemVer before invalid
		case b.v != nil:
			return false
		default:
			return a.raw > b.raw
		}
	})
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.raw
	}
	return out
}

func orDefault(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

func hostOf(u string) string {
	s := u
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	if i := strings.IndexByte(s, '/'); i >= 0 {
		s = s[:i]
	}
	return s
}
