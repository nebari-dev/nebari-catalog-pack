// Package registry enumerates available software packs from an OCI registry.
//
// Quay is the default backend. The canonical discovery path is:
//
//  1. GET <api>/repository?namespace=<ns>&public=true  -> repositories
//     keep those whose name starts with "<chart-prefix>/"; the remainder is
//     the pack/chart name.
//  2. GET <oci>/<ns>/<prefix>/<name>/tags/list          -> versions (SemVer tags)
//
// The OCI /v2/_catalog endpoint is intentionally NOT used: quay.io returns an
// empty list for it, so it cannot enumerate repositories.
package registry

// Pack is one installable software pack discovered in the registry.
type Pack struct {
	// Name is the chart/repo name, e.g. "nebari-lgtm-pack".
	Name string `json:"name"`
	// Versions are the available tags, newest first.
	Versions []string `json:"versions"`
	// Latest is Versions[0] when present.
	Latest string `json:"latest"`
	// OCIRef is the pullable OCI reference, e.g.
	// oci://quay.io/nebari/charts/nebari-lgtm-pack.
	OCIRef string `json:"ociRef"`

	// The following are best-effort metadata, populated from a pack's
	// pack-metadata.yaml when available (see metadata.go). They drive the card
	// presentation; all are optional.
	DisplayName string `json:"displayName,omitempty"`
	Description string `json:"description,omitempty"`
	Icon        string `json:"icon,omitempty"`
	Category    string `json:"category,omitempty"`
	Level       string `json:"level,omitempty"` // experimental|alpha|beta|ga
	Homepage    string `json:"homepage,omitempty"`
	Deprecated  bool   `json:"deprecated,omitempty"`
}

// Title returns the best human label for the pack.
func (p Pack) Title() string {
	if p.DisplayName != "" {
		return p.DisplayName
	}
	return p.Name
}

// quayRepoList is the shape of GET /api/v1/repository.
type quayRepoList struct {
	Repositories []quayRepo `json:"repositories"`
	NextPage     string     `json:"next_page"`
}

type quayRepo struct {
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
	IsPublic    bool   `json:"is_public"`
	Description string `json:"description"`
}

// ociTagList is the shape of GET /v2/<repo>/tags/list.
type ociTagList struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}
