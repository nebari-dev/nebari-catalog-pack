package gitops

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	httpauth "github.com/go-git/go-git/v5/plumbing/transport/http"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

// RepoConfig describes the GitOps repository and how to authenticate to it.
type RepoConfig struct {
	URL         string // https://, ssh://, or file://
	Branch      string // default main
	AppsDir     string // directory inside the repo for Application files, e.g. "clusters/aws/apps"
	Token       string // PAT for https remotes
	SSHKeyPath  string // private key for ssh remotes
	AuthorName  string
	AuthorEmail string
}

// CommitResult reports the outcome of an install commit.
type CommitResult struct {
	File       string // path within the repo, e.g. clusters/aws/apps/lgtm-pack.yaml
	CommitHash string // empty if nothing changed
	Pushed     bool
	NoChange   bool
}

// Writer commits Application manifests into the GitOps repo.
type Writer struct {
	cfg RepoConfig
}

// NewWriter builds a Writer.
func NewWriter(cfg RepoConfig) *Writer {
	if cfg.Branch == "" {
		cfg.Branch = "main"
	}
	if cfg.AppsDir == "" {
		cfg.AppsDir = "apps"
	}
	return &Writer{cfg: cfg}
}

// Commit clones the repo into a fresh temp dir, writes filename under AppsDir
// with content, commits, and pushes. The temp clone is always removed.
//
// The Application file's metadata.name must match filename's stem, which the
// caller (Builder.ApplicationFilename) guarantees.
func (w *Writer) Commit(ctx context.Context, filename, content, message string) (*CommitResult, error) {
	auth, err := w.auth()
	if err != nil {
		return nil, err
	}

	dir, err := os.MkdirTemp("", "catalog-gitops-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)

	repo, err := git.PlainCloneContext(ctx, dir, false, &git.CloneOptions{
		URL:           w.cfg.URL,
		Auth:          auth,
		ReferenceName: plumbing.NewBranchReferenceName(w.cfg.Branch),
		SingleBranch:  true,
		Depth:         1,
	})
	if err != nil {
		return nil, fmt.Errorf("clone %s: %w", w.cfg.URL, err)
	}

	rel := filepath.ToSlash(filepath.Join(w.cfg.AppsDir, filename))
	abs := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return nil, err
	}
	// 0644 so argocd-repo-server (uid 999) can read it when the repo is a
	// file:// path shared into the cluster.
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		return nil, err
	}

	wt, err := repo.Worktree()
	if err != nil {
		return nil, err
	}
	if _, err := wt.Add(rel); err != nil {
		return nil, fmt.Errorf("git add %s: %w", rel, err)
	}

	status, err := wt.Status()
	if err != nil {
		return nil, err
	}
	res := &CommitResult{File: rel}
	if status.IsClean() {
		res.NoChange = true
		return res, nil
	}

	hash, err := wt.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  orDefault(w.cfg.AuthorName, "nebari-catalog"),
			Email: orDefault(w.cfg.AuthorEmail, "catalog@nebari.dev"),
			When:  commitTime(),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	res.CommitHash = hash.String()

	// We always clone into a temp dir, so the commit must be pushed back to the
	// remote — including file:// remotes (a bare repo elsewhere). Auth is nil for
	// local transports; see auth().
	if err := repo.PushContext(ctx, &git.PushOptions{
		Auth:       auth,
		RefSpecs:   []config.RefSpec{config.RefSpec(fmt.Sprintf("refs/heads/%s:refs/heads/%s", w.cfg.Branch, w.cfg.Branch))},
		RemoteName: "origin",
	}); err != nil {
		return res, fmt.Errorf("push: %w", err)
	}
	res.Pushed = true
	return res, nil
}

func (w *Writer) auth() (transport.AuthMethod, error) {
	switch {
	case isLocal(w.cfg.URL):
		return nil, nil
	case w.cfg.SSHKeyPath != "":
		return gitssh.NewPublicKeysFromFile("git", w.cfg.SSHKeyPath, "")
	case w.cfg.Token != "":
		// GitHub/GitLab accept the PAT as the password with any non-empty user.
		return &httpauth.BasicAuth{Username: "x-access-token", Password: w.cfg.Token}, nil
	default:
		return nil, nil
	}
}

func commitTime() time.Time { return time.Now() }

func isLocal(u string) bool {
	return strings.HasPrefix(u, "file://") || strings.HasPrefix(u, "/") || strings.HasPrefix(u, "./")
}

func orDefault(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}
