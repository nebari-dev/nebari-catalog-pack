// Package ui holds the templ view models and components for the catalog gallery.
package ui

import "github.com/nebari-dev/nebari-catalog-pack/internal/registry"

// PageData is the model for the full gallery page.
type PageData struct {
	Title          string
	BasePath       string // URL prefix for links/actions, e.g. "/catalog"
	Packs          []registry.Pack
	InstallEnabled bool   // false when GitOps is not configured
	DryRun         bool   // installs only preview
	RegistryRef    string // human label of the registry source, e.g. quay.io/nebari/charts
	Error          string // top-level error banner, if any
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
