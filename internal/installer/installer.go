// Package installer orchestrates installing a pack: resolve the chart source,
// render an ArgoCD Application, commit it to the GitOps repo, then nudge ArgoCD
// and wait for the child Application to become healthy.
package installer

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/nebari-dev/nebari-catalog-pack/internal/argocd"
	"github.com/nebari-dev/nebari-catalog-pack/internal/config"
	"github.com/nebari-dev/nebari-catalog-pack/internal/gitops"
	"github.com/nebari-dev/nebari-catalog-pack/internal/registry"
)

// Result captures the outcome of an install attempt.
type Result struct {
	Pack       string
	Version    string
	Source     string // oci | git
	DryRun     bool
	Manifest   string
	File       string
	CommitHash string
	Pushed     bool
	NoChange   bool
	Health     string
	Sync       string
	Summary    string
}

// Installer ties together manifest generation, git, and ArgoCD.
type Installer struct {
	cfg     *config.Config
	builder gitops.Builder
	writer  *gitops.Writer // nil when GitOps not configured
	argo    *argocd.Client // nil when ArgoCD disabled/unreachable

	WaitTimeout  time.Duration
	WaitInterval time.Duration
}

// New builds an Installer. writer may be nil (read-only) and argo may be nil
// (no in-cluster nudge).
func New(cfg *config.Config, writer *gitops.Writer, argo *argocd.Client) *Installer {
	return &Installer{
		cfg:    cfg,
		writer: writer,
		argo:   argo,
		builder: gitops.Builder{
			Project:   cfg.Install.Project,
			Namespace: cfg.Install.Namespace,
			SyncWave:  cfg.Install.SyncWave,
			ManagedBy: cfg.Install.ManagedBy,
			PartOf:    cfg.Install.PartOf,
			Domain:    cfg.Install.Domain,
		},
		WaitTimeout:  3 * time.Minute,
		WaitInterval: 5 * time.Second,
	}
}

// Enabled reports whether installs can be committed.
func (i *Installer) Enabled() bool { return i.writer != nil }

// InstalledStatus returns the status of every ArgoCD Application, keyed by name
// (which equals the pack name). Best-effort: returns nil when ArgoCD is not
// configured/reachable or the list fails, so callers degrade gracefully.
func (i *Installer) InstalledStatus(ctx context.Context) map[string]argocd.Status {
	if i.argo == nil {
		return nil
	}
	m, err := i.argo.List(ctx)
	if err != nil {
		return nil
	}
	return m
}

// buildRequest resolves a pack + version into an InstallRequest.
func (i *Installer) buildRequest(p registry.Pack, version string) gitops.InstallRequest {
	if version == "" {
		version = p.Latest
	}
	req := gitops.InstallRequest{
		PackName:    p.Name,
		AppName:     appNameFor(p.Name),
		Version:     version,
		DisplayName: p.DisplayName,
		Description: p.Description,
		Icon:        p.Icon,
		Category:    p.Category,
	}

	useOCI := i.cfg.Install.PreferOCI && version != ""
	if useOCI {
		req.Source = gitops.SourceOCI
		req.OCIRepoURL = strings.TrimPrefix(i.cfg.Registry.OCIPullBase, "oci://")
		req.Chart = p.Name
	} else {
		req.Source = gitops.SourceGit
		req.GitRepoURL = fmt.Sprintf(i.cfg.Install.GitTemplate, p.Name)
		req.GitPath = "."
		req.GitRevision = orDefault(version, "main")
	}
	return req
}

// Install resolves, renders, and (unless dry-run) commits + nudges.
func (i *Installer) Install(ctx context.Context, p registry.Pack, version string) (*Result, error) {
	req := i.buildRequest(p, version)
	manifest, err := i.builder.Render(req)
	if err != nil {
		return nil, fmt.Errorf("render manifest: %w", err)
	}

	res := &Result{
		Pack:     p.Name,
		Version:  req.Version,
		Source:   string(req.Source),
		Manifest: manifest,
	}

	if i.cfg.DryRun || i.writer == nil {
		res.DryRun = true
		if i.writer == nil {
			res.Summary = "GitOps repo not configured — preview only."
		} else {
			res.Summary = "Dry-run: manifest rendered, nothing committed."
		}
		return res, nil
	}

	filename := i.builder.ApplicationFilename(req)
	msg := fmt.Sprintf("Install %s (%s) via nebari-catalog-pack", p.Name, req.Version)
	commit, err := i.writer.Commit(ctx, filename, manifest, msg)
	if err != nil {
		return nil, fmt.Errorf("commit to gitops repo: %w", err)
	}
	res.File = commit.File
	res.CommitHash = commit.CommitHash
	res.Pushed = commit.Pushed
	res.NoChange = commit.NoChange

	if commit.NoChange {
		res.Summary = "Already present in the GitOps repo — no change."
	} else {
		res.Summary = fmt.Sprintf("Committed %s.", commit.File)
	}

	// Nudge ArgoCD and observe status, best-effort.
	if i.argo != nil {
		if err := i.argo.Refresh(ctx); err == nil {
			appName := appNameFor(p.Name)
			if st, werr := i.argo.WaitReady(ctx, appName, i.WaitTimeout, i.WaitInterval); werr == nil {
				res.Health, res.Sync = st.Health, st.Sync
				res.Summary += " Application is Healthy."
			} else {
				st2, _ := i.argo.Get(ctx, appName)
				res.Health, res.Sync = st2.Health, st2.Sync
				res.Summary += " Syncing (check ArgoCD for progress)."
			}
		}
	}
	return res, nil
}

// appNameFor derives the ArgoCD Application name from a chart name. The chart
// name is used verbatim so the apps/<name>.yaml file and Application name match.
func appNameFor(chart string) string { return chart }

func orDefault(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}
