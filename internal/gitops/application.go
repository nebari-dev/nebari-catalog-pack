// Package gitops generates ArgoCD Application manifests for software packs and
// commits them into the GitOps repository ArgoCD watches.
//
// The manifest shape mirrors the established nebari-packs convention observed
// in openteams-ai/nic-deploy: project "foundational", part-of "nebari-packs",
// sync-wave "7", destination namespace "nebari-system" opted in via
// nebari.dev/managed=true, and a Helm values block carrying the nebariapp +
// landingPage contract the nebari-operator consumes.
package gitops

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

// SourceType selects where the chart is pulled from.
type SourceType string

const (
	// SourceOCI pulls the chart from the OCI registry (quay.io/nebari/charts).
	SourceOCI SourceType = "oci"
	// SourceGit pulls the chart from a git repo (github.com/nebari-dev/<name>).
	SourceGit SourceType = "git"
)

// InstallRequest is a resolved request to install one pack.
type InstallRequest struct {
	// PackName is the chart/registry name, e.g. "nebari-lgtm-pack".
	PackName string
	// AppName is the ArgoCD Application name and the apps/<AppName>.yaml file
	// stem. Defaults to PackName.
	AppName string
	// Version is the chart version (OCI tag / git tag). For git sources this is
	// used as targetRevision; if empty, GitRevision is used.
	Version string
	// Source selects OCI vs git.
	Source SourceType

	// Hostname for the NebariApp (routing/TLS). When empty and Domain is set on
	// the builder, it is derived as <AppName>.<Domain>.
	Hostname string
	// ExtraValues is an optional Helm values block (YAML, already indented at
	// column 0) merged under spec.source.helm.values. The nebariapp block is
	// always generated; ExtraValues is appended verbatim for chart-specific keys.
	ExtraValues string
	// ValuesOverride, when set, REPLACES the generated values block entirely and
	// is written verbatim as spec.source.helm.values. It is the user-edited YAML
	// from the values drawer (which starts from DefaultValues). Takes precedence
	// over the generated block + ExtraValues.
	ValuesOverride string

	// OCIRepoURL / Chart are used for SourceOCI (e.g. "quay.io/nebari/charts").
	OCIRepoURL string
	Chart      string
	// GitRepoURL / GitPath / GitRevision are used for SourceGit.
	GitRepoURL  string
	GitPath     string
	GitRevision string

	// LandingPage drives the operator's app gallery entry. Optional.
	DisplayName string
	Description string
	Icon        string
	Category    string
}

// Builder renders InstallRequests into Application YAML using shared conventions.
type Builder struct {
	Project   string
	Namespace string
	SyncWave  string
	ManagedBy string
	PartOf    string
	Domain    string
}

// ApplicationFilename returns the file name (no directory) for a request.
func (b Builder) ApplicationFilename(r InstallRequest) string {
	return appName(r) + ".yaml"
}

// Render produces the Application manifest YAML for a request.
func (b Builder) Render(r InstallRequest) (string, error) {
	name := appName(r)
	if err := validateName(name); err != nil {
		return "", err
	}

	hostname := b.hostnameFor(r)

	data := struct {
		Builder
		R           InstallRequest
		Name        string
		Hostname    string
		ValuesBlock string
	}{
		Builder:  b,
		R:        r,
		Name:     name,
		Hostname: hostname,
	}
	values := b.valuesYAML(r, hostname)
	if strings.TrimSpace(r.ValuesOverride) != "" {
		values = strings.TrimRight(r.ValuesOverride, "\n") + "\n"
	}
	data.ValuesBlock = indent(values, 8)

	var buf bytes.Buffer
	if err := appTemplate.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// hostnameFor resolves the NebariApp hostname: explicit, else <app>.<domain>.
func (b Builder) hostnameFor(r InstallRequest) string {
	if r.Hostname != "" {
		return r.Hostname
	}
	if b.Domain != "" {
		return appName(r) + "." + b.Domain
	}
	return ""
}

// DefaultValues returns the generated values block (column 0) that prefills the
// values drawer, so the user edits from the catalog's nebariapp/landingPage
// contract rather than a blank slate.
func (b Builder) DefaultValues(r InstallRequest) string {
	return b.valuesYAML(r, b.hostnameFor(r))
}

// valuesYAML builds the Helm values block (nebariapp + landingPage + extras).
func (b Builder) valuesYAML(r InstallRequest, hostname string) string {
	var sb strings.Builder
	sb.WriteString("nebariapp:\n")
	sb.WriteString("  enabled: true\n")
	if hostname != "" {
		fmt.Fprintf(&sb, "  hostname: %s\n", hostname)
	}
	sb.WriteString("  landingPage:\n")
	sb.WriteString("    enabled: true\n")
	fmt.Fprintf(&sb, "    displayName: %q\n", orStr(r.DisplayName, r.PackName))
	if r.Description != "" {
		fmt.Fprintf(&sb, "    description: %q\n", r.Description)
	}
	if r.Icon != "" {
		fmt.Fprintf(&sb, "    icon: %q\n", r.Icon)
	}
	if r.Category != "" {
		fmt.Fprintf(&sb, "    category: %q\n", r.Category)
	}
	if extra := strings.TrimRight(r.ExtraValues, "\n"); extra != "" {
		sb.WriteString(extra)
		sb.WriteString("\n")
	}
	return sb.String()
}

func appName(r InstallRequest) string {
	if r.AppName != "" {
		return r.AppName
	}
	return r.PackName
}

// validateName rejects path-traversal and invalid Application names. Mirrors the
// add-software-pack action's guard.
func validateName(name string) error {
	if name == "" {
		return fmt.Errorf("application name is empty")
	}
	if strings.ContainsAny(name, "/\\") || name == "." || name == ".." || strings.Contains(name, "..") {
		return fmt.Errorf("invalid application name %q", name)
	}
	return nil
}

func orStr(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

// indent indents every non-empty line of s by n spaces.
func indent(s string, n int) string {
	pad := strings.Repeat(" ", n)
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		lines[i] = pad + l
	}
	return strings.Join(lines, "\n")
}

var appTemplate = template.Must(template.New("app").Parse(`apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: {{ .Name }}
  namespace: argocd
  labels:
    app.kubernetes.io/part-of: {{ .PartOf }}
    app.kubernetes.io/managed-by: {{ .ManagedBy }}
  annotations:
    argocd.argoproj.io/sync-wave: "{{ .SyncWave }}"
  finalizers:
    - resources-finalizer.argocd.argoproj.io
spec:
  project: {{ .Project }}

  source:
{{- if eq .R.Source "git" }}
    repoURL: {{ .R.GitRepoURL }}
    targetRevision: {{ .R.GitRevision }}
    path: {{ .R.GitPath }}
{{- else }}
    repoURL: {{ .R.OCIRepoURL }}
    chart: {{ .R.Chart }}
    targetRevision: {{ .R.Version }}
{{- end }}
    helm:
      releaseName: {{ .Name }}
      values: |
{{ .ValuesBlock }}

  destination:
    server: https://kubernetes.default.svc
    namespace: {{ .Namespace }}

  syncPolicy:
    managedNamespaceMetadata:
      labels:
        nebari.dev/managed: "true"
    automated:
      prune: true
      selfHeal: true
      allowEmpty: false
    syncOptions:
      - CreateNamespace=true
      - ServerSideApply=true
      - SkipDryRunOnMissingResource=true
    retry:
      limit: 5
      backoff:
        duration: 5s
        factor: 2
        maxDuration: 3m
`))
