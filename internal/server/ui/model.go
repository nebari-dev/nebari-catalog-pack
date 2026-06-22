// Package ui holds the templ view models and components for the catalog gallery.
package ui

import "github.com/nebari-dev/nebari-catalog-pack/internal/registry"

// PageData is the model for the full gallery page.
type PageData struct {
	Title          string
	BasePath       string          // URL prefix for links/actions, e.g. "/catalog"
	Packs          []registry.Pack // already filtered/sorted for the current view
	Total          int             // total packs before filtering (for the count label)
	InstallEnabled bool            // false when GitOps is not configured
	DryRun         bool            // installs only preview
	RegistryRef    string          // human label of the registry source, e.g. quay.io/nebari/charts
	Error          string          // top-level error banner, if any
	Filters        Filters         // current toolbar state + available options
	// Installed maps pack name -> its ArgoCD status for packs already present
	// in the cluster. Empty when ArgoCD is not configured/reachable.
	Installed map[string]InstalledInfo
}

// InstalledInfo is the cluster state of an already-installed pack.
type InstalledInfo struct {
	Health string // Healthy, Progressing, Degraded, Missing, ...
	Sync   string // Synced, OutOfSync, ...
}

// Filters captures the gallery's search/filter/sort state plus the option
// lists the toolbar renders. The grid is filtered server-side and swapped in
// via htmx, so no client-side JavaScript is involved.
type Filters struct {
	Query      string   // free-text search
	Category   string   // selected category ("" = all)
	Level      string   // selected maturity level ("" = all)
	Sort       string   // "name" (default) | "maturity" | "category"
	Categories []string // distinct categories across all packs
	Levels     []string // distinct levels across all packs
}

// InstallResult is the model for the htmx install-response fragment.
type InstallResult struct {
	Pack       string
	Version    string
	OK         bool
	DryRun     bool
	Message    string // success summary or error
	File       string // committed file path
	CommitHash string
	Manifest   string // rendered Application YAML (shown on dry-run / preview)
	Health     string
	Sync       string
}
