package gitops

import (
	"strings"
	"testing"

	"sigs.k8s.io/yaml"
)

func testBuilder() Builder {
	return Builder{
		Project:   "foundational",
		Namespace: "nebari-system",
		SyncWave:  "7",
		ManagedBy: "nebari-catalog-pack",
		PartOf:    "nebari-packs",
		Domain:    "nebari.example.com",
	}
}

func TestRenderOCIIsValidYAML(t *testing.T) {
	b := testBuilder()
	out, err := b.Render(InstallRequest{
		PackName:    "nebari-lgtm-pack",
		AppName:     "lgtm-pack",
		Version:     "0.1.3",
		Source:      SourceOCI,
		OCIRepoURL:  "quay.io/nebari/charts",
		Chart:       "nebari-lgtm-pack",
		DisplayName: "Grafana",
		Description: "Observability",
		Icon:        "https://grafana.com/x.svg",
		Category:    "Observability",
	})
	if err != nil {
		t.Fatal(err)
	}

	var m map[string]any
	if err := yaml.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("rendered manifest is not valid YAML: %v\n---\n%s", err, out)
	}
	for _, want := range []string{
		"name: lgtm-pack",
		"app.kubernetes.io/part-of: nebari-packs",
		`argocd.argoproj.io/sync-wave: "7"`,
		"repoURL: quay.io/nebari/charts",
		"chart: nebari-lgtm-pack",
		"targetRevision: 0.1.3",
		"namespace: nebari-system",
		"hostname: lgtm-pack.nebari.example.com",
		`nebari.dev/managed: "true"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestRenderGitSource(t *testing.T) {
	b := testBuilder()
	out, err := b.Render(InstallRequest{
		PackName:    "nebari-lgtm-pack",
		Source:      SourceGit,
		GitRepoURL:  "https://github.com/nebari-dev/nebari-lgtm-pack.git",
		GitPath:     ".",
		GitRevision: "main",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "repoURL: https://github.com/nebari-dev/nebari-lgtm-pack.git") {
		t.Errorf("git repoURL missing:\n%s", out)
	}
	if strings.Contains(out, "chart:") {
		t.Errorf("git source must not emit a chart field:\n%s", out)
	}
}

func TestRenderRejectsTraversal(t *testing.T) {
	b := testBuilder()
	if _, err := b.Render(InstallRequest{PackName: "../evil"}); err == nil {
		t.Fatal("expected error for path-traversal name")
	}
}
