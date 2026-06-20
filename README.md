# nebari-catalog-pack

A Nebari **software pack** that turns your cluster's pack registry into a
browsable gallery — and lets you **install a pack with one click** by committing
an ArgoCD `Application` straight into your GitOps repository.

It is a pack that installs packs. It ships the way every other pack does (a Helm
chart carrying a `NebariApp`), and it drives the same GitOps install path that
Nebari's tooling uses under the hood.

## What it does

- **Browse** every pack published to an OCI registry — defaults to
  `quay.io/nebari/charts`, the registry behind
  [`nebari-dev/helm-repository`](https://github.com/nebari-dev/helm-repository).
  Each pack renders as a card (name, description, icon, category, version),
  enriched from the pack's `pack-metadata.yaml` when present.
- **Install** a pack into the cluster by writing `apps/<name>.yaml` — an ArgoCD
  `Application` — into your GitOps repo and committing it. ArgoCD's app-of-apps
  discovers the new file and syncs it. The catalog then nudges ArgoCD and
  reports the rolled-out app's health.

The differentiator is that last step: the gallery is not just a viewer, it is an
installer that mutates your GitOps repo through ArgoCD's own model.

See [`docs/architecture.md`](docs/architecture.md) for the full design and a
sequence diagram of the install path.

## Quick start (local, read-only)

Browse the live Nebari registry with no cluster and no credentials:

```bash
make -f dev/Makefile run
# open http://localhost:8080/
```

This runs in dry-run/read-only mode: it lists packs from `quay.io/nebari/charts`
and, on "Preview", renders the exact `Application` it *would* commit — without
touching any repo.

## Deploy as a pack

```bash
helm install catalog ./chart \
  --namespace nebari-system --create-namespace \
  --set nebariapp.enabled=true \
  --set nebariapp.hostname=catalog.nebari.example.com \
  --set catalog.install.domain=nebari.example.com \
  --set catalog.gitops.repoURL=https://github.com/your-org/your-gitops.git \
  --set catalog.gitops.path=clusters/your-cluster \
  --set catalog.gitops.tokenSecret.name=gitops-token
```

`gitops-token` is an existing `Secret` with a `token` key holding a git PAT that
has push access to the GitOps repo. Without a `gitops.repoURL` the catalog still
deploys, but installs are disabled (read-only gallery).

The chart defaults `nebariapp.auth.enabled=true` restricted to the `admin`
group — this tool can install software cluster-wide, so keep it behind auth.

### Key configuration

| Value | Default | Purpose |
| --- | --- | --- |
| `catalog.registry.namespace` | `nebari` | registry org/namespace to browse |
| `catalog.registry.chartPrefix` | `charts` | OCI repo prefix for charts |
| `catalog.gitops.repoURL` | `""` | GitOps repo to commit Applications into (empty = read-only) |
| `catalog.gitops.path` | `""` | cluster sub-dir; apps go to `<path>/apps/` |
| `catalog.gitops.tokenSecret.name` | `""` | Secret holding a git PAT |
| `catalog.gitops.sshKeySecret.name` | `""` | Secret holding an SSH key (alternative to PAT) |
| `catalog.install.preferOCI` | `true` | source generated Apps from OCI vs git |
| `catalog.install.domain` | `""` | base domain for installed packs' hostnames |
| `catalog.argocd.rootApp` | `nebari-root` | app-of-apps to nudge after a commit |
| `catalog.dryRun` | `false` | preview manifests without committing |

Full list in [`chart/values.yaml`](chart/values.yaml). Every value maps to a
`CATALOG_*` environment variable (see [`internal/config`](internal/config/config.go)).

## Development

```bash
make -f dev/Makefile generate   # regenerate templ components
make -f dev/Makefile test       # go test ./...
make -f dev/Makefile vet
make -f dev/Makefile image      # docker build
make -f dev/Makefile helm-lint
```

The UI is server-rendered with [templ](https://templ.guide) and
[htmx](https://htmx.org) (both vendored — no CDN, no node toolchain). Generated
`*_templ.go` files are committed; CI verifies they are up to date.

## How a pack reaches the registry

This chart publishes itself the same way other Nebari packs do: on a GitHub
release, [`sync-helm-chart.yml`](.github/workflows/sync-helm-chart.yml) opens a
PR in `nebari-dev/helm-repository`; when merged, that repo packages and pushes
the chart to `quay.io/nebari/charts/nebari-catalog-pack` — where this very
catalog will then list it.

## License

Apache 2.0 — see [LICENSE](LICENSE).
