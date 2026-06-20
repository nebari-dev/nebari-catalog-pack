// Package config loads the catalog server configuration from the environment.
//
// Every field has a sane default so the server boots for read-only browsing
// with no configuration at all. Installing packs requires the GitOps section
// to be filled in (a repo URL plus auth); when it is absent the server still
// renders the gallery but reports install as unavailable.
//
// Following the NIC convention (pkg/git in nebari-infrastructure-core), secret
// material is never embedded in config values that get serialized — the git
// token is read from the environment at process start and kept only in memory.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config is the fully-resolved runtime configuration.
type Config struct {
	// HTTP
	ListenAddr string // address the server binds, e.g. ":8080"
	BasePath   string // URL prefix when served behind a path-routed gateway, e.g. "/catalog"

	// Registry discovery (quay by default)
	Registry RegistryConfig

	// GitOps install target
	GitOps GitOpsConfig

	// ArgoCD nudge/poll
	ArgoCD ArgoCDConfig

	// Conventions baked into generated Application manifests
	Install InstallConfig

	// DryRun renders and previews the Application manifest but never commits.
	DryRun bool

	// Demo serves a fixed, offline set of packs instead of querying the
	// registry. Used for deterministic screenshots and quick local demos.
	Demo bool
}

// RegistryConfig describes how to enumerate packs from an OCI/Quay registry.
type RegistryConfig struct {
	// APIBase is the Quay REST API base, used to list repositories.
	// Default: https://quay.io/api/v1
	APIBase string
	// OCIBase is the OCI Distribution v2 base, used to list tags per repo.
	// Default: https://quay.io/v2
	OCIBase string
	// Namespace is the registry org/namespace. Default: nebari
	Namespace string
	// ChartPrefix selects which repos under the namespace are charts.
	// Quay stores Helm charts at <namespace>/<prefix>/<chart>. Default: charts
	ChartPrefix string
	// OCIPullBase is the base used in generated Application OCI sources,
	// e.g. oci://quay.io/nebari/charts. Derived from the above by default.
	OCIPullBase string
}

// GitOpsConfig describes the git repository ArgoCD watches.
type GitOpsConfig struct {
	// RepoURL is the GitOps repo, https://, ssh:// or file://. Empty disables install.
	RepoURL string
	// Branch tracked by ArgoCD. Default: main
	Branch string
	// Path is the subdirectory inside the repo for this cluster, e.g.
	// "clusters/aws-nic-deploy". Applications are written to <Path>/apps/<name>.yaml.
	Path string
	// Token is a git PAT (read from TokenEnv). Mutually exclusive with SSHKeyPath.
	Token string
	// SSHKeyPath points at a private key file for ssh:// remotes.
	SSHKeyPath string
	// AuthorName/AuthorEmail stamp commits. Request-scoped, never persisted to
	// the repo's .git/config (mirrors the add-software-pack action).
	AuthorName  string
	AuthorEmail string
}

// Configured reports whether installs are possible.
func (g GitOpsConfig) Configured() bool { return g.RepoURL != "" }

// ArgoCDConfig controls the post-commit reconcile nudge.
type ArgoCDConfig struct {
	// Enabled turns on the in-cluster refresh nudge + status poll. When false
	// (e.g. running outside the cluster) installs still commit; ArgoCD picks
	// the change up on its own schedule.
	Enabled bool
	// Namespace ArgoCD runs in. Default: argocd
	Namespace string
	// RootApp is the app-of-apps to nudge after a commit. Default: nebari-root
	RootApp string
}

// InstallConfig holds conventions stamped into generated Application manifests.
// Defaults match the established nebari-packs pattern observed in nic-deploy.
type InstallConfig struct {
	Project     string // spec.project. Default: foundational
	Namespace   string // spec.destination.namespace. Default: nebari-system
	SyncWave    string // sync-wave annotation. Default: "7"
	ManagedBy   string // managed-by label. Default: nebari-catalog-pack
	PartOf      string // part-of label. Default: nebari-packs
	Domain      string // base domain for NebariApp hostnames, e.g. nebari.example.com
	GitTemplate string // git source fallback, %s = chart name. Default: https://github.com/nebari-dev/%s.git
	// PreferOCI chooses the OCI source over the git source when a pack is
	// available in the registry. Default: true.
	PreferOCI bool
}

// Load resolves configuration from environment variables with defaults.
func Load() (*Config, error) {
	c := &Config{
		ListenAddr: env("CATALOG_LISTEN", ":8080"),
		BasePath:   strings.TrimRight(env("CATALOG_BASE_PATH", ""), "/"),
		DryRun:     boolEnv("CATALOG_DRY_RUN", false),
		Demo:       boolEnv("CATALOG_DEMO", false),
		Registry: RegistryConfig{
			APIBase:     strings.TrimRight(env("CATALOG_REGISTRY_API", "https://quay.io/api/v1"), "/"),
			OCIBase:     strings.TrimRight(env("CATALOG_REGISTRY_OCI", "https://quay.io/v2"), "/"),
			Namespace:   env("CATALOG_REGISTRY_NAMESPACE", "nebari"),
			ChartPrefix: env("CATALOG_REGISTRY_CHART_PREFIX", "charts"),
			OCIPullBase: strings.TrimRight(env("CATALOG_REGISTRY_OCI_PULL_BASE", ""), "/"),
		},
		GitOps: GitOpsConfig{
			RepoURL:     env("CATALOG_GITOPS_REPO_URL", ""),
			Branch:      env("CATALOG_GITOPS_BRANCH", "main"),
			Path:        strings.Trim(env("CATALOG_GITOPS_PATH", ""), "/"),
			SSHKeyPath:  env("CATALOG_GITOPS_SSH_KEY_PATH", ""),
			AuthorName:  env("CATALOG_GIT_AUTHOR_NAME", "nebari-catalog"),
			AuthorEmail: env("CATALOG_GIT_AUTHOR_EMAIL", "catalog@nebari.dev"),
		},
		ArgoCD: ArgoCDConfig{
			Enabled:   boolEnv("CATALOG_ARGOCD_ENABLED", true),
			Namespace: env("CATALOG_ARGOCD_NAMESPACE", "argocd"),
			RootApp:   env("CATALOG_ARGOCD_ROOT_APP", "nebari-root"),
		},
		Install: InstallConfig{
			Project:     env("CATALOG_APP_PROJECT", "foundational"),
			Namespace:   env("CATALOG_APP_NAMESPACE", "nebari-system"),
			SyncWave:    env("CATALOG_APP_SYNC_WAVE", "7"),
			ManagedBy:   env("CATALOG_APP_MANAGED_BY", "nebari-catalog-pack"),
			PartOf:      env("CATALOG_APP_PART_OF", "nebari-packs"),
			Domain:      env("CATALOG_DOMAIN", ""),
			GitTemplate: env("CATALOG_PACK_GIT_TEMPLATE", "https://github.com/nebari-dev/%s.git"),
			PreferOCI:   boolEnv("CATALOG_PREFER_OCI", true),
		},
	}

	// Git token is read from a named env var (default CATALOG_GIT_TOKEN) so the
	// var name can be customized to whatever the deployment mounts.
	tokenEnv := env("CATALOG_GIT_TOKEN_ENV", "CATALOG_GIT_TOKEN")
	c.GitOps.Token = os.Getenv(tokenEnv)

	// Derive the OCI pull base from registry coordinates if not set explicitly.
	if c.Registry.OCIPullBase == "" {
		host := hostOf(c.Registry.OCIBase)
		c.Registry.OCIPullBase = fmt.Sprintf("oci://%s/%s/%s", host, c.Registry.Namespace, c.Registry.ChartPrefix)
	}

	if c.GitOps.Token != "" && c.GitOps.SSHKeyPath != "" {
		return nil, fmt.Errorf("config: set only one of %s or CATALOG_GITOPS_SSH_KEY_PATH", tokenEnv)
	}

	return c, nil
}

// AppsDir returns the directory (relative to repo root) Applications live in.
func (c *Config) AppsDir() string {
	if c.GitOps.Path == "" {
		return "apps"
	}
	return c.GitOps.Path + "/apps"
}

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return def
}

func boolEnv(key string, def bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok {
		return def
	}
	b, err := strconv.ParseBool(strings.TrimSpace(v))
	if err != nil {
		return def
	}
	return b
}

// hostOf extracts the host from a URL like https://quay.io/v2 -> quay.io.
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
