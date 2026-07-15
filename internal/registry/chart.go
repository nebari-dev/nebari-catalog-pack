package registry

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// helmChartLayer is the media type Helm pushes the chart tarball as via OCI.
const helmChartLayer = "application/vnd.cncf.helm.chart.content.v1.tar+gzip"

// chartValuesCache memoizes pulled values.yaml by "<chart>:<version>"; chart
// blobs are ~hundreds of KB so re-pulling on every drawer open is wasteful.
type chartValuesCache struct {
	mu sync.Mutex
	m  map[string]string
}

func (c *chartValuesCache) get(k string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.m[k]
	return v, ok
}

func (c *chartValuesCache) put(k, v string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.m == nil {
		c.m = map[string]string{}
	}
	c.m[k] = v
}

// ChartValues pulls a chart's default values.yaml from the OCI registry,
// best-effort. Returns ("", nil) when the chart/version is not in the registry
// so callers can fall back to the generated contract values.
func (c *Client) ChartValues(ctx context.Context, chart, version string) (string, error) {
	if version == "" {
		tags, err := c.listTags(ctx, chart)
		if err != nil || len(tags) == 0 {
			return "", nil
		}
		version = sortVersionsDesc(tags)[0]
	}
	key := chart + ":" + version
	if v, ok := c.chartCache.get(key); ok {
		return v, nil
	}

	repo := fmt.Sprintf("%s/%s/%s", c.namespace, c.chartPrefix, chart)
	manifestURL := fmt.Sprintf("%s/%s/manifests/%s", c.ociBase, repo, version)
	raw, err := c.ociGet(ctx, manifestURL, []string{
		"application/vnd.oci.image.manifest.v1+json",
		"application/vnd.docker.distribution.manifest.v2+json",
	})
	if err != nil {
		return "", nil // not published as an OCI chart; fall back
	}
	var manifest struct {
		Layers []struct {
			MediaType string `json:"mediaType"`
			Digest    string `json:"digest"`
		} `json:"layers"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return "", nil
	}
	digest := ""
	for _, l := range manifest.Layers {
		if l.MediaType == helmChartLayer {
			digest = l.Digest
			break
		}
	}
	if digest == "" {
		return "", nil
	}

	blob, err := c.ociGet(ctx, fmt.Sprintf("%s/%s/blobs/%s", c.ociBase, repo, digest), nil)
	if err != nil {
		return "", nil
	}
	values, err := valuesFromChartTGZ(blob)
	if err != nil {
		return "", nil
	}
	c.chartCache.put(key, values)
	return values, nil
}

// valuesFromChartTGZ extracts the top-level <chart>/values.yaml from a Helm
// chart tarball.
func valuesFromChartTGZ(tgz []byte) (string, error) {
	gz, err := gzip.NewReader(bytes.NewReader(tgz))
	if err != nil {
		return "", err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		// chart tarballs are rooted at <chart>/, so values.yaml is at depth 1.
		if strings.HasSuffix(hdr.Name, "/values.yaml") && strings.Count(strings.Trim(hdr.Name, "/"), "/") == 1 {
			b, err := io.ReadAll(tr)
			if err != nil {
				return "", err
			}
			return string(b), nil
		}
	}
	return "", fmt.Errorf("values.yaml not found in chart")
}

// ociGet does a registry GET, performing the standard anonymous Bearer-token
// dance on a 401 (parse WWW-Authenticate -> fetch token -> retry).
func (c *Client) ociGet(ctx context.Context, urlStr string, accept []string) ([]byte, error) {
	do := func(token string) (*http.Response, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
		if err != nil {
			return nil, err
		}
		for _, a := range accept {
			req.Header.Add("Accept", a)
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		return c.http.Do(req)
	}

	resp, err := do("")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		challenge := resp.Header.Get("WWW-Authenticate")
		_ = resp.Body.Close()
		token, err := c.fetchToken(ctx, challenge)
		if err != nil {
			return nil, err
		}
		if resp, err = do(token); err != nil {
			return nil, err
		}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: status %d", urlStr, resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 32<<20)) // 32 MiB guard
}

// fetchToken resolves an anonymous pull token from a Bearer WWW-Authenticate
// challenge: `Bearer realm="...",service="...",scope="..."`.
func (c *Client) fetchToken(ctx context.Context, challenge string) (string, error) {
	if !strings.HasPrefix(challenge, "Bearer ") {
		return "", fmt.Errorf("unexpected auth challenge")
	}
	params := map[string]string{}
	for _, part := range strings.Split(strings.TrimPrefix(challenge, "Bearer "), ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) == 2 {
			params[kv[0]] = strings.Trim(kv[1], `"`)
		}
	}
	realm := params["realm"]
	if realm == "" {
		return "", fmt.Errorf("no realm in auth challenge")
	}
	u, err := url.Parse(realm)
	if err != nil {
		return "", err
	}
	q := u.Query()
	if s := params["service"]; s != "" {
		q.Set("service", s)
	}
	if s := params["scope"]; s != "" {
		q.Set("scope", s)
	}
	u.RawQuery = q.Encode()

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token endpoint: status %d", resp.StatusCode)
	}
	var tok struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return "", err
	}
	if tok.Token != "" {
		return tok.Token, nil
	}
	return tok.AccessToken, nil
}
