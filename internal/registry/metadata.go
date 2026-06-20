package registry

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"sigs.k8s.io/yaml"
)

// packMetadata mirrors the pack-metadata.yaml convention consumed by
// nebari-dev/software-pack-dashboard. All fields are optional; we read only the
// subset that drives card presentation.
type packMetadata struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Category    string `json:"category"`
	Level       string `json:"level"`
	Owner       string `json:"owner"`
	Homepage    string `json:"homepage"`
	Deprecated  bool   `json:"deprecated"`
}

// Enricher fetches pack-metadata.yaml for packs to enrich card display.
// It is best-effort: missing or malformed metadata is silently ignored.
type Enricher struct {
	// URLTemplate is a printf template with one %s for the pack name, resolving
	// to a raw pack-metadata.yaml, e.g.
	// https://raw.githubusercontent.com/nebari-dev/%s/main/pack-metadata.yaml
	URLTemplate string
	http        *http.Client
}

// NewEnricher builds an Enricher. An empty template disables enrichment.
func NewEnricher(urlTemplate string) *Enricher {
	return &Enricher{
		URLTemplate: urlTemplate,
		http:        &http.Client{Timeout: 8 * time.Second},
	}
}

// Enrich fills in display metadata on each pack in place, concurrently.
func (e *Enricher) Enrich(ctx context.Context, packs []Pack) {
	if e == nil || e.URLTemplate == "" {
		return
	}
	var wg sync.WaitGroup
	for i := range packs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			md, err := e.fetch(ctx, packs[i].Name)
			if err != nil || md == nil {
				return
			}
			merge(&packs[i], md)
		}(i)
	}
	wg.Wait()
}

func (e *Enricher) fetch(ctx context.Context, name string) (*packMetadata, error) {
	url := fmt.Sprintf(e.URLTemplate, name)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := e.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var md packMetadata
	if err := yaml.Unmarshal(body, &md); err != nil {
		return nil, err
	}
	return &md, nil
}

// merge applies non-empty metadata fields onto a pack.
func merge(p *Pack, md *packMetadata) {
	if md.DisplayName != "" {
		p.DisplayName = md.DisplayName
	}
	if md.Description != "" {
		p.Description = md.Description
	}
	if md.Icon != "" {
		p.Icon = md.Icon
	}
	if md.Category != "" {
		p.Category = md.Category
	}
	if md.Level != "" {
		p.Level = md.Level
	}
	if md.Homepage != "" {
		p.Homepage = md.Homepage
	}
	p.Deprecated = p.Deprecated || md.Deprecated
}
