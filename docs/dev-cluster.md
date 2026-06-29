# Local test cluster

Two ways to run the catalog on a real cluster locally, plus the no-cluster path.
All targets are in [`dev/Makefile`](../dev/Makefile) (run from the repo root) and
wrap [`dev/cluster.sh`](../dev/cluster.sh).

## Tier 0 — no cluster (UI/dev loop)

```bash
# Terminal 1: the API + demo data
CATALOG_DEMO=true CATALOG_DRY_RUN=true go run ./cmd/catalog
# Terminal 2: the SPA with hot reload (proxy /api to :8080 or run the binary)
npm --prefix web run dev
```

No Kubernetes. Fastest loop for frontend work; serves fixed offline fixtures.

## Tier 1 — quick smoke (k3d + Helm, read-only)

A k3d cluster + `helm install` of the chart standalone (NebariApp disabled, no
ArgoCD). ~1 minute. Confirms the chart deploys and the gallery renders against a
real cluster — it does **not** exercise the install path (no operator/ArgoCD).

```bash
make -f dev/Makefile cluster-quick        # create k3d + build/import image + helm install
kubectl -n nebari-system port-forward svc/nebari-catalog-pack 8080:80
# open http://localhost:8080/  (read-only gallery against the live registry)
make -f dev/Makefile cluster-quick-down   # delete the cluster
```

**Prereqs:** Docker, k3d, helm, kubectl.

## Tier 2 — full platform (kind via NIC) + the real install path

Stands up a complete local Nebari platform with NIC's local provider (kind:
cert-manager, Envoy Gateway, Keycloak, ArgoCD, nebari-operator, MetalLB) and an
auto GitOps repo at `/tmp/nebari-gitops-nebari-catalog` watched by the
`nebari-root` app-of-apps. Then deploys the catalog onto it so you can exercise
the real path: browse -> values drawer -> commit an `Application` -> ArgoCD
reconciles it.

```bash
make -f dev/Makefile cluster-up           # nic deploy -f dev/local-config.yaml (~15 min first boot)
make -f dev/Makefile catalog-up           # run the catalog on the host (default), open http://localhost:8080/
make -f dev/Makefile cluster-status       # ArgoCD apps + catalog pods
make -f dev/Makefile cluster-down         # nic destroy
```

**Prereqs:** Docker, kind, [`nic`](https://github.com/nebari-dev/nebari-infrastructure-core),
helm, kubectl, Go (host mode), Node 20 (builds the SPA). Add `*.nebari.local`
to `/etc/hosts` (or use the gateway IP) to reach operator-routed hostnames.

### Two ways to deploy the catalog (`catalog-up`)

- **`host` (default)** — runs the Go binary on your machine against the
  platform's GitOps repo (`file://`) + kubeconfig. Instant rebuild/restart, no
  image build; mirrors the integration workflow's runner-side catalog.
  ```bash
  make -f dev/Makefile catalog-up            # == catalog-up MODE=host
  ```
- **`in-cluster`** — builds the image, `kind load`s it, and registers the
  catalog as a pack in the GitOps repo (faithful deployment). Set `KIND_CLUSTER`
  if NIC names the kind cluster differently than the project.
  ```bash
  make -f dev/Makefile catalog-up MODE=in-cluster
  ```

## CI parity

The same Tier-2 flow runs in CI via
[`integration.yml`](../.github/workflows/integration.yml), which uses
[`action-nebari-sandbox`](https://github.com/nebari-dev/action-nebari-sandbox)
(k3d instead of kind) to boot the platform and assert the catalog deploys and
installs a pack.
