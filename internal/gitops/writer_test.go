package gitops

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// seedBareRepo creates a bare repo with one commit on the given branch and
// returns its file:// URL. Mirrors a real GitOps remote.
func seedBareRepo(t *testing.T, branch string) string {
	t.Helper()
	bare := t.TempDir()
	if _, err := git.PlainInit(bare, true); err != nil {
		t.Fatal(err)
	}
	work := t.TempDir()
	r, err := git.PlainInit(work, false)
	if err != nil {
		t.Fatal(err)
	}
	wt, _ := r.Worktree()
	if err := os.WriteFile(filepath.Join(work, "README.md"), []byte("seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := wt.Add("README.md"); err != nil {
		t.Fatal(err)
	}
	if _, err := wt.Commit("seed", &git.CommitOptions{
		Author: &object.Signature{Name: "t", Email: "t@t", When: commitTime()},
	}); err != nil {
		t.Fatal(err)
	}
	// Point the requested branch at the seed commit.
	headRef, _ := r.Head()
	branchRef := plumbing.NewHashReference(plumbing.NewBranchReferenceName(branch), headRef.Hash())
	if err := r.Storer.SetReference(branchRef); err != nil {
		t.Fatal(err)
	}
	if _, err := r.CreateRemote(&config.RemoteConfig{Name: "origin", URLs: []string{bare}}); err != nil {
		t.Fatal(err)
	}
	if err := r.Push(&git.PushOptions{
		RemoteName: "origin",
		RefSpecs:   []config.RefSpec{config.RefSpec("refs/heads/" + branch + ":refs/heads/" + branch)},
	}); err != nil {
		t.Fatal(err)
	}
	return "file://" + bare
}

func TestWriterCommitsAndPushes(t *testing.T) {
	url := seedBareRepo(t, "trunk")
	w := NewWriter(RepoConfig{URL: url, Branch: "trunk", AppsDir: "clusters/test/apps"})

	res, err := w.Commit(context.Background(), "demo.yaml", "kind: Application\n", "install demo")
	if err != nil {
		t.Fatal(err)
	}
	if res.NoChange || res.CommitHash == "" || !res.Pushed {
		t.Fatalf("expected a pushed commit, got %+v", res)
	}
	if res.File != "clusters/test/apps/demo.yaml" {
		t.Errorf("unexpected file path %q", res.File)
	}

	// Re-cloning the remote must show the committed file — proving the push.
	verify := t.TempDir()
	if _, err := git.PlainClone(verify, false, &git.CloneOptions{URL: url, ReferenceName: plumbing.NewBranchReferenceName("trunk"), SingleBranch: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(verify, "clusters/test/apps/demo.yaml")); err != nil {
		t.Fatalf("committed file missing from remote: %v", err)
	}

	// A second identical write is a no-op.
	res2, err := w.Commit(context.Background(), "demo.yaml", "kind: Application\n", "install demo again")
	if err != nil {
		t.Fatal(err)
	}
	if !res2.NoChange {
		t.Errorf("expected NoChange on identical re-commit, got %+v", res2)
	}
}
