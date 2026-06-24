package server

import (
	"encoding/json"
	"net/http"

	"github.com/nebari-dev/nebari-catalog-pack/internal/registry"
)

// packDTO is the JSON shape the SPA consumes for a catalog card/row.
type packDTO struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Chart       string   `json:"chart"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Maturity    string   `json:"maturity"` // experimental|alpha|beta|ga
	Version     string   `json:"version"`
	Versions    []string `json:"versions"`
	Icon        string   `json:"icon,omitempty"`
	Homepage    string   `json:"homepage,omitempty"`
	Deprecated  bool     `json:"deprecated,omitempty"`
	Installed   bool     `json:"installed"`
	Health      string   `json:"health,omitempty"`
	Sync        string   `json:"sync,omitempty"`
}

// gitopsDTO drives the SPA's GitOps context bar.
type gitopsDTO struct {
	InstallEnabled bool   `json:"installEnabled"`
	DryRun         bool   `json:"dryRun"`
	Repo           string `json:"repo"`
	Branch         string `json:"branch"`
	RootApp        string `json:"rootApp"`
	ArgoCDEnabled  bool   `json:"argocdEnabled"`
	ArgoCDHealth   string `json:"argocdHealth,omitempty"`
	RegistryRef    string `json:"registryRef"`
	InstalledCount int    `json:"installedCount"`
}

type installReq struct {
	Pack    string `json:"pack"`
	Version string `json:"version"`
	DryRun  bool   `json:"dryRun"`
	// Values, when non-empty, replaces the generated Helm values block (the
	// user-edited YAML from the values drawer).
	Values string `json:"values"`
}

type installResp struct {
	OK         bool   `json:"ok"`
	DryRun     bool   `json:"dryRun"`
	Pack       string `json:"pack"`
	Version    string `json:"version"`
	Message    string `json:"message"`
	File       string `json:"file,omitempty"`
	CommitHash string `json:"commitHash,omitempty"`
	Manifest   string `json:"manifest,omitempty"`
	Health     string `json:"health,omitempty"`
	Sync       string `json:"sync,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// handleAPIPacks returns the enriched pack list merged with installed-state.
func (s *Server) handleAPIPacks(w http.ResponseWriter, r *http.Request) {
	packs, err := s.packs(r.Context())
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "registry unavailable: " + err.Error()})
		return
	}
	installed := s.installedStatus(r.Context())
	out := make([]packDTO, 0, len(packs))
	for _, p := range packs {
		st, ok := installed[p.Name]
		out = append(out, packDTO{
			ID:          p.Name,
			Name:        p.Title(),
			Chart:       p.Name,
			Description: p.Description,
			Category:    p.Category,
			Maturity:    p.Level,
			Version:     p.Latest,
			Versions:    p.Versions,
			Icon:        p.Icon,
			Homepage:    p.Homepage,
			Deprecated:  p.Deprecated,
			Installed:   ok && st.Exists,
			Health:      st.Health,
			Sync:        st.Sync,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"packs": out})
}

// handleAPIInstall renders (dry-run) or commits an Application for a pack.
func (s *Server) handleAPIInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req installReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, installResp{OK: false, Message: "bad request"})
		return
	}
	packs, err := s.packs(r.Context())
	if err != nil {
		writeJSON(w, http.StatusBadGateway, installResp{OK: false, Pack: req.Pack, Message: "registry unavailable"})
		return
	}
	var pack *registry.Pack
	for i := range packs {
		if packs[i].Name == req.Pack {
			pack = &packs[i]
			break
		}
	}
	if pack == nil {
		writeJSON(w, http.StatusNotFound, installResp{OK: false, Pack: req.Pack, Message: "unknown pack"})
		return
	}

	res, err := s.inst.Install(r.Context(), *pack, req.Version, req.DryRun, req.Values)
	if err != nil {
		s.log.Error("install", "pack", req.Pack, "err", err)
		writeJSON(w, http.StatusInternalServerError, installResp{OK: false, Pack: req.Pack, Message: err.Error()})
		return
	}
	// Bust the installed-state cache so a real install reflects immediately.
	if !res.DryRun {
		s.instMu.Lock()
		s.instCache = nil
		s.instMu.Unlock()
	}
	writeJSON(w, http.StatusOK, installResp{
		OK:         true,
		DryRun:     res.DryRun,
		Pack:       res.Pack,
		Version:    res.Version,
		Message:    res.Summary,
		File:       res.File,
		CommitHash: res.CommitHash,
		Manifest:   res.Manifest,
		Health:     res.Health,
		Sync:       res.Sync,
	})
}

// handleAPIValues returns the generated default Helm values for a pack+version,
// used to prefill the values-override editor in the drawer.
func (s *Server) handleAPIValues(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("pack")
	packs, err := s.packs(r.Context())
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "registry unavailable"})
		return
	}
	for i := range packs {
		if packs[i].Name == name {
			writeJSON(w, http.StatusOK, map[string]string{
				"values": s.inst.DefaultValues(packs[i], r.URL.Query().Get("version")),
			})
			return
		}
	}
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown pack"})
}

// handleAPIGitops returns the GitOps/ArgoCD context for the SPA's status bar.
func (s *Server) handleAPIGitops(w http.ResponseWriter, r *http.Request) {
	dto := gitopsDTO{
		InstallEnabled: s.inst.Enabled(),
		DryRun:         s.cfg.DryRun,
		Repo:           s.cfg.GitOps.RepoURL,
		Branch:         s.cfg.GitOps.Branch,
		RootApp:        s.cfg.ArgoCD.RootApp,
		ArgoCDEnabled:  s.argo != nil,
		RegistryRef:    s.cfg.Registry.Namespace + "/" + s.cfg.Registry.ChartPrefix,
	}
	if s.argo != nil {
		if st, err := s.argo.Get(r.Context(), s.cfg.ArgoCD.RootApp); err == nil && st.Exists {
			dto.ArgoCDHealth = st.Health
		}
	}
	dto.InstalledCount = len(s.installedStatus(r.Context()))
	if s.cfg.Demo {
		dto.Repo, dto.Branch, dto.ArgoCDEnabled, dto.ArgoCDHealth = "nebari-dev/gitops", "main", true, "Healthy"
	}
	writeJSON(w, http.StatusOK, dto)
}
