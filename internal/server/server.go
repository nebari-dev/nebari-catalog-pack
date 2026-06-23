// Package server exposes the catalog as a JSON API and serves the embedded
// React single-page app. The SPA (web/dist) talks to /api/* for the pack list,
// installs, and GitOps/ArgoCD context.
package server

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/nebari-dev/nebari-catalog-pack/internal/argocd"
	"github.com/nebari-dev/nebari-catalog-pack/internal/config"
	"github.com/nebari-dev/nebari-catalog-pack/internal/installer"
	"github.com/nebari-dev/nebari-catalog-pack/internal/registry"
	"github.com/nebari-dev/nebari-catalog-pack/web"
)

// Server is the HTTP application.
type Server struct {
	cfg      *config.Config
	reg      *registry.Client
	enricher *registry.Enricher
	inst     *installer.Installer
	argo     *argocd.Client // nil when ArgoCD disabled/unreachable
	log      *slog.Logger

	cacheTTL time.Duration
	mu       sync.Mutex
	cached   []registry.Pack
	cachedAt time.Time
	cacheErr error

	// installed-state cache (short TTL so repeated polls don't hammer the API).
	instMu       sync.Mutex
	instCache    map[string]argocd.Status
	instCachedAt time.Time
}

// New builds a Server. argo may be nil (no in-cluster ArgoCD).
func New(cfg *config.Config, reg *registry.Client, enricher *registry.Enricher, inst *installer.Installer, argo *argocd.Client, log *slog.Logger) *Server {
	return &Server{
		cfg:      cfg,
		reg:      reg,
		enricher: enricher,
		inst:     inst,
		argo:     argo,
		log:      log,
		cacheTTL: 60 * time.Second,
	}
}

// Handler returns the configured HTTP handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	base := s.cfg.BasePath

	mux.HandleFunc(base+"/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc(base+"/api/packs", s.handleAPIPacks)
	mux.HandleFunc(base+"/api/install", s.handleAPIInstall)
	mux.HandleFunc(base+"/api/gitops", s.handleAPIGitops)
	mux.Handle(base+"/", s.spaHandler())
	return mux
}

// spaHandler serves the embedded SPA. When the frontend has not been built it
// returns a clear message instead of a confusing 404.
func (s *Server) spaHandler() http.Handler {
	dist, ok := web.Dist()
	if !ok {
		s.log.Warn("SPA not built — serving placeholder (run `npm --prefix web run build`)")
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == s.cfg.BasePath+"/" || r.URL.Path == s.cfg.BasePath {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				_, _ = w.Write([]byte("<!doctype html><title>Nebari Pack Catalog</title>" +
					"<p>Frontend not built. Run <code>npm --prefix web run build</code>.</p>"))
				return
			}
			http.NotFound(w, r)
		})
	}
	fileServer := http.FileServer(http.FS(dist))
	if base := s.cfg.BasePath; base != "" {
		return http.StripPrefix(base, fileServer)
	}
	return fileServer
}

// packs returns the pack list, served from a short-lived cache.
func (s *Server) packs(ctx context.Context) ([]registry.Pack, error) {
	if s.cfg.Demo {
		return registry.Fixtures(), nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cached != nil && time.Since(s.cachedAt) < s.cacheTTL {
		return s.cached, s.cacheErr
	}
	packs, err := s.reg.List(ctx)
	if err != nil {
		if s.cached != nil {
			return s.cached, nil // serve stale on error
		}
		s.cacheErr = err
		return nil, err
	}
	s.enricher.Enrich(ctx, packs)
	s.cached, s.cachedAt, s.cacheErr = packs, time.Now(), nil
	return packs, nil
}

// installedStatus returns which packs are already installed (by ArgoCD
// Application name), from a short-lived cache. Best-effort: nil when ArgoCD is
// unavailable. In demo mode it returns a fixed entry so the UI shows the state.
func (s *Server) installedStatus(ctx context.Context) map[string]argocd.Status {
	if s.cfg.Demo {
		return map[string]argocd.Status{
			"nebari-data-science-pack": {Exists: true, Health: "Healthy", Sync: "Synced"},
		}
	}
	s.instMu.Lock()
	defer s.instMu.Unlock()
	if s.instCache != nil && time.Since(s.instCachedAt) < 15*time.Second {
		return s.instCache
	}
	m := s.inst.InstalledStatus(ctx)
	if m == nil {
		return nil
	}
	s.instCache, s.instCachedAt = m, time.Now()
	return m
}
