// Package server wires the registry, installer, and templ UI into an HTTP
// service: a gallery of pack cards plus an htmx-driven install endpoint.
package server

import (
	"context"
	"embed"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/nebari-dev/nebari-catalog-pack/internal/config"
	"github.com/nebari-dev/nebari-catalog-pack/internal/installer"
	"github.com/nebari-dev/nebari-catalog-pack/internal/registry"
	"github.com/nebari-dev/nebari-catalog-pack/internal/server/ui"
)

//go:embed static
var staticFS embed.FS

// Server is the HTTP application.
type Server struct {
	cfg      *config.Config
	reg      *registry.Client
	enricher *registry.Enricher
	inst     *installer.Installer
	log      *slog.Logger
	cacheTTL time.Duration
	mu       sync.Mutex
	cached   []registry.Pack
	cachedAt time.Time
	cacheErr error
}

// New builds a Server.
func New(cfg *config.Config, reg *registry.Client, enricher *registry.Enricher, inst *installer.Installer, log *slog.Logger) *Server {
	return &Server{
		cfg:      cfg,
		reg:      reg,
		enricher: enricher,
		inst:     inst,
		log:      log,
		cacheTTL: 60 * time.Second,
	}
}

// Handler returns the configured HTTP handler (with base-path routing).
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	base := s.cfg.BasePath

	mux.HandleFunc(base+"/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle(base+"/static/", http.StripPrefix(base+"/", http.FileServer(http.FS(staticFS))))
	mux.HandleFunc(base+"/install", s.handleInstall)
	mux.HandleFunc(base+"/", s.handleIndex)
	return mux
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
		// Serve stale on error if we have any.
		if s.cached != nil {
			return s.cached, nil
		}
		s.cacheErr = err
		return nil, err
	}
	s.enricher.Enrich(ctx, packs)
	s.cached, s.cachedAt, s.cacheErr = packs, time.Now(), nil
	return packs, nil
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != s.cfg.BasePath+"/" && r.URL.Path != s.cfg.BasePath {
		http.NotFound(w, r)
		return
	}
	data := ui.PageData{
		Title:          "Nebari Pack Catalog",
		BasePath:       s.cfg.BasePath,
		InstallEnabled: s.inst.Enabled(),
		DryRun:         s.cfg.DryRun,
		RegistryRef:    s.cfg.Registry.Namespace + "/" + s.cfg.Registry.ChartPrefix,
	}
	packs, err := s.packs(r.Context())
	if err != nil {
		data.Error = "Could not reach the registry: " + err.Error()
		s.log.Error("list packs", "err", err)
	}
	data.Packs = packs

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := ui.Layout(data).Render(r.Context(), w); err != nil {
		s.log.Error("render index", "err", err)
	}
}

func (s *Server) handleInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		s.renderResult(w, r, ui.InstallResult{OK: false, Message: "bad request"})
		return
	}
	packName := r.FormValue("pack")
	version := r.FormValue("version")

	packs, err := s.packs(r.Context())
	if err != nil {
		s.renderResult(w, r, ui.InstallResult{Pack: packName, OK: false, Message: "registry unavailable"})
		return
	}
	var pack *registry.Pack
	for i := range packs {
		if packs[i].Name == packName {
			pack = &packs[i]
			break
		}
	}
	if pack == nil {
		s.renderResult(w, r, ui.InstallResult{Pack: packName, OK: false, Message: "unknown pack"})
		return
	}

	res, err := s.inst.Install(r.Context(), *pack, version)
	if err != nil {
		s.log.Error("install", "pack", packName, "err", err)
		s.renderResult(w, r, ui.InstallResult{Pack: packName, OK: false, Message: err.Error()})
		return
	}
	s.renderResult(w, r, ui.InstallResult{
		Pack:       res.Pack,
		Version:    res.Version,
		OK:         true,
		DryRun:     res.DryRun,
		Message:    res.Summary,
		File:       res.File,
		CommitHash: res.CommitHash,
		Manifest:   manifestForDisplay(res),
		Health:     res.Health,
		Sync:       res.Sync,
	})
}

// manifestForDisplay shows the rendered manifest on dry-run/preview only.
func manifestForDisplay(res *installer.Result) string {
	if res.DryRun {
		return res.Manifest
	}
	return ""
}

func (s *Server) renderResult(w http.ResponseWriter, r *http.Request, res ui.InstallResult) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := ui.Result(res).Render(r.Context(), w); err != nil {
		s.log.Error("render result", "err", err)
	}
}
