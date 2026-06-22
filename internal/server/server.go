// Package server wires the registry, installer, and templ UI into an HTTP
// service: a gallery of pack cards plus an htmx-driven install endpoint.
package server

import (
	"context"
	"embed"
	"log/slog"
	"net/http"
	"sort"
	"strings"
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
	mux.HandleFunc(base+"/gallery", s.handleGallery)
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
	data := s.buildPageData(r)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := ui.Layout(data).Render(r.Context(), w); err != nil {
		s.log.Error("render index", "err", err)
	}
}

// handleGallery returns just the gallery grid for the current filters, swapped
// in by htmx from the toolbar controls.
func (s *Server) handleGallery(w http.ResponseWriter, r *http.Request) {
	data := s.buildPageData(r)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := ui.Gallery(data).Render(r.Context(), w); err != nil {
		s.log.Error("render gallery", "err", err)
	}
}

// buildPageData fetches the pack list, applies the request's filters/sort, and
// assembles the view model shared by the full page and the gallery fragment.
func (s *Server) buildPageData(r *http.Request) ui.PageData {
	q := r.URL.Query()
	f := ui.Filters{
		Query:    strings.TrimSpace(q.Get("q")),
		Category: q.Get("category"),
		Level:    q.Get("level"),
		Sort:     q.Get("sort"),
	}
	data := ui.PageData{
		Title:          "Nebari Pack Catalog",
		BasePath:       s.cfg.BasePath,
		InstallEnabled: s.inst.Enabled(),
		DryRun:         s.cfg.DryRun,
		RegistryRef:    s.cfg.Registry.Namespace + "/" + s.cfg.Registry.ChartPrefix,
		Filters:        f,
	}

	all, err := s.packs(r.Context())
	if err != nil {
		data.Error = "Could not reach the registry: " + err.Error()
		s.log.Error("list packs", "err", err)
		return data
	}

	data.Total = len(all)
	f.Categories = distinctField(all, func(p registry.Pack) string { return p.Category })
	f.Levels = distinctField(all, func(p registry.Pack) string { return p.Level })
	data.Filters = f

	filtered := filterPacks(all, f.Query, f.Category, f.Level)
	sortPacks(filtered, f.Sort)
	data.Packs = filtered
	return data
}

// filterPacks keeps packs matching the (optional) category, level, and
// free-text query. The query matches name, display name, description, category.
func filterPacks(packs []registry.Pack, query, category, level string) []registry.Pack {
	query = strings.ToLower(query)
	out := make([]registry.Pack, 0, len(packs))
	for _, p := range packs {
		if category != "" && p.Category != category {
			continue
		}
		if level != "" && p.Level != level {
			continue
		}
		if query != "" && !packMatches(p, query) {
			continue
		}
		out = append(out, p)
	}
	return out
}

func packMatches(p registry.Pack, q string) bool {
	return strings.Contains(strings.ToLower(p.Name), q) ||
		strings.Contains(strings.ToLower(p.DisplayName), q) ||
		strings.Contains(strings.ToLower(p.Description), q) ||
		strings.Contains(strings.ToLower(p.Category), q)
}

// sortPacks orders in place: by maturity (most mature first), by category, or
// by title (default).
func sortPacks(packs []registry.Pack, by string) {
	switch by {
	case "maturity":
		sort.SliceStable(packs, func(i, j int) bool {
			return levelRank(packs[i].Level) > levelRank(packs[j].Level)
		})
	case "category":
		sort.SliceStable(packs, func(i, j int) bool {
			if packs[i].Category != packs[j].Category {
				return packs[i].Category < packs[j].Category
			}
			return strings.ToLower(packs[i].Title()) < strings.ToLower(packs[j].Title())
		})
	default:
		sort.SliceStable(packs, func(i, j int) bool {
			return strings.ToLower(packs[i].Title()) < strings.ToLower(packs[j].Title())
		})
	}
}

func levelRank(level string) int {
	switch strings.ToLower(level) {
	case "ga", "stable":
		return 4
	case "beta":
		return 3
	case "alpha":
		return 2
	case "experimental":
		return 1
	default:
		return 0
	}
}

// distinctField returns the sorted, de-duplicated non-empty values of key.
func distinctField(packs []registry.Pack, key func(registry.Pack) string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, p := range packs {
		v := key(p)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	sort.Strings(out)
	return out
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
