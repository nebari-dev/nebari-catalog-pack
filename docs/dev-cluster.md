# Local test cluster

Two ways to work on the catalog; the full-platform test runs in CI.

## No cluster (UI / dev loop)

```bash
# Terminal 1: the API + demo data
CATALOG_DEMO=true CATALOG_DRY_RUN=true go run ./cmd/catalog
# Terminal 2: the SPA with hot reload
npm --prefix web run dev
```

No Kubernetes; fastest loop for frontend work (serves fixed offline fixtures).

## Local cluster (kind + Helm, standalone)

A [kind](https://kind.sigs.k8s.io) cluster + `helm install` of the chart with
the NebariApp disabled (read-only gallery, no operator/ArgoCD). Confirms the
chart deploys and the UI renders on a real cluster. Mirrors the other Nebari
packs' `dev/Makefile`.

```bash
make -f dev/Makefile cluster-create    # kind create cluster
make -f dev/Makefile cluster-deploy    # build image -> kind load -> helm install
make -f dev/Makefile cluster-forward   # port-forward -> http://localhost:8080/
make -f dev/Makefile cluster-delete    # tear down
```

**Prereqs:** Docker, kind, helm, kubectl, plus Node 20 + Go (the image build
bundles the SPA + binary).

> This standalone path does **not** exercise the install flow (committing an
> ArgoCD `Application` and reconciling it) — that needs the operator + ArgoCD.

## Full platform (CI)

The end-to-end install path against a real Nebari platform is covered by
[`integration.yml`](../.github/workflows/integration.yml), which boots the
foundational stack with
[`action-nebari-sandbox`](https://github.com/nebari-dev/action-nebari-sandbox)
(platform profile), deploys the catalog via the `add-software-pack` sub-action
(`.github/argo-apps/nebari-catalog-pack.yaml`), and asserts it installs a pack.
Running that full stack locally is intentionally left to CI — it's heavy and
the sandbox action exists so packs don't each maintain platform setup scripts.
