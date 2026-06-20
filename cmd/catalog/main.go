// Command catalog runs the Nebari pack catalog server: a gallery of software
// packs discovered from an OCI registry, each installable into the cluster's
// GitOps repository as an ArgoCD Application.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nebari-dev/nebari-catalog-pack/internal/argocd"
	"github.com/nebari-dev/nebari-catalog-pack/internal/config"
	"github.com/nebari-dev/nebari-catalog-pack/internal/gitops"
	"github.com/nebari-dev/nebari-catalog-pack/internal/installer"
	"github.com/nebari-dev/nebari-catalog-pack/internal/registry"
	"github.com/nebari-dev/nebari-catalog-pack/internal/server"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Load()
	if err != nil {
		log.Error("load config", "err", err)
		os.Exit(1)
	}

	reg := registry.New(registry.Options{
		APIBase:     cfg.Registry.APIBase,
		OCIBase:     cfg.Registry.OCIBase,
		Namespace:   cfg.Registry.Namespace,
		ChartPrefix: cfg.Registry.ChartPrefix,
		OCIPullBase: cfg.Registry.OCIPullBase,
	})
	enricher := registry.NewEnricher(
		envOr("CATALOG_METADATA_URL_TEMPLATE",
			"https://raw.githubusercontent.com/nebari-dev/%s/main/pack-metadata.yaml"),
	)

	// GitOps writer is optional: absent it, the server is read-only.
	var writer *gitops.Writer
	if cfg.GitOps.Configured() {
		writer = gitops.NewWriter(gitops.RepoConfig{
			URL:         cfg.GitOps.RepoURL,
			Branch:      cfg.GitOps.Branch,
			AppsDir:     cfg.AppsDir(),
			Token:       cfg.GitOps.Token,
			SSHKeyPath:  cfg.GitOps.SSHKeyPath,
			AuthorName:  cfg.GitOps.AuthorName,
			AuthorEmail: cfg.GitOps.AuthorEmail,
		})
		log.Info("gitops configured", "repo", cfg.GitOps.RepoURL, "appsDir", cfg.AppsDir())
	} else {
		log.Warn("gitops not configured — running read-only (set CATALOG_GITOPS_REPO_URL to enable installs)")
	}

	// ArgoCD client is optional: absent it, installs still commit but are not nudged.
	var argo *argocd.Client
	if cfg.GitOps.Configured() && cfg.ArgoCD.Enabled {
		argo, err = argocd.New(cfg.ArgoCD.Namespace, cfg.ArgoCD.RootApp)
		if err != nil {
			log.Warn("argocd client unavailable — installs will commit without a reconcile nudge", "err", err)
			argo = nil
		}
	}

	inst := installer.New(cfg, writer, argo)
	srv := server.New(cfg, reg, enricher, inst, log)

	httpServer := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Info("catalog server listening", "addr", cfg.ListenAddr, "basePath", cfg.BasePath, "dryRun", cfg.DryRun)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server error", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)
}

func envOr(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return def
}
