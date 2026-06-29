#!/usr/bin/env bash
#
# Local test-cluster helper for nebari-catalog-pack. Two tiers:
#
#   Tier 1 (quick)    k3d cluster-only + `helm install` the chart standalone
#                     (NebariApp disabled, read-only). Fast (~1 min) smoke test
#                     of the chart + UI on a real cluster, no ArgoCD/operator.
#
#   Tier 2 (platform) a full local Nebari platform via NIC's kind provider, then
#                     the catalog deployed onto it so the real install path
#                     (browse -> values drawer -> commit Application -> ArgoCD
#                     reconciles) can be exercised end to end.
#
# Usage:
#   dev/cluster.sh quick            # Tier 1: up + helm install + how to reach it
#   dev/cluster.sh quick-down       # Tier 1: delete the k3d cluster
#   dev/cluster.sh up               # Tier 2: deploy the local Nebari platform
#   dev/cluster.sh catalog [host|in-cluster]   # Tier 2: deploy/run the catalog
#   dev/cluster.sh status           # show ArgoCD apps + pods
#   dev/cluster.sh down             # Tier 2: destroy the platform
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
QUICK_CLUSTER="${QUICK_CLUSTER:-catalog-quick}"
PROJECT="${PROJECT:-nebari-catalog}"          # NIC project_name in dev/local-config.yaml
GITOPS_DIR="${GITOPS_DIR:-/tmp/nebari-gitops-${PROJECT}}"
KIND_CLUSTER="${KIND_CLUSTER:-${PROJECT}}"    # adjust if NIC names the kind cluster differently
IMAGE="${IMAGE:-nebari-catalog-pack:dev}"
CONFIG="${ROOT}/dev/local-config.yaml"
PORT="${PORT:-8080}"

log()  { printf '\033[36m==>\033[0m %s\n' "$*"; }
die()  { printf '\033[31merror:\033[0m %s\n' "$*" >&2; exit 1; }
need() { for c in "$@"; do command -v "$c" >/dev/null 2>&1 || die "missing required tool: $c"; done; }

build_image() {
  log "Building image ${IMAGE} (SPA + Go, multi-stage)"
  docker build -t "${IMAGE}" "${ROOT}"
}

build_web() {
  log "Building the SPA (web/dist) so the Go binary embeds it"
  ( cd "${ROOT}/web" && npm ci && npm run build )
}

# ── Tier 1 ────────────────────────────────────────────────────────────────
cmd_quick() {
  need docker k3d helm kubectl
  if k3d cluster list -o json 2>/dev/null | grep -q "\"name\":\"${QUICK_CLUSTER}\""; then
    log "k3d cluster ${QUICK_CLUSTER} exists; reusing it"
  else
    log "Creating k3d cluster ${QUICK_CLUSTER}"
    k3d cluster create "${QUICK_CLUSTER}" --wait --timeout 120s
  fi
  build_image
  log "Importing ${IMAGE} into the cluster"
  k3d image import "${IMAGE}" -c "${QUICK_CLUSTER}"
  log "helm install (standalone: NebariApp disabled, read-only gallery)"
  helm upgrade --install nebari-catalog-pack "${ROOT}/chart" \
    --namespace nebari-system --create-namespace \
    --set image.repository=nebari-catalog-pack \
    --set image.tag=dev \
    --set image.pullPolicy=IfNotPresent \
    --set nebariapp.enabled=false \
    --set rbac.create=false \
    --set catalog.gitops.repoURL="" \
    --set catalog.argocd.enabled=false
  kubectl -n nebari-system rollout status deploy/nebari-catalog-pack --timeout=120s
  log "Ready. Browse the gallery (read-only) with:"
  echo "    kubectl -n nebari-system port-forward svc/nebari-catalog-pack ${PORT}:80"
  echo "    open http://localhost:${PORT}/"
}

cmd_quick_down() {
  need k3d
  k3d cluster delete "${QUICK_CLUSTER}"
}

# ── Tier 2 ────────────────────────────────────────────────────────────────
cmd_up() {
  need docker kind nic kubectl
  log "Deploying a local Nebari platform via NIC (kind) — first boot ~15 min"
  nic deploy -f "${CONFIG}" --timeout 20m
  log "Platform up. GitOps repo: ${GITOPS_DIR}"
  log "Deploy the catalog onto it with: dev/cluster.sh catalog"
}

cmd_down() {
  need nic
  nic destroy -f "${CONFIG}"
}

cmd_catalog() {
  local mode="${1:-host}"
  [ -d "${GITOPS_DIR}" ] || die "GitOps dir ${GITOPS_DIR} not found — run 'dev/cluster.sh up' first"
  # go-git pushes to the checked-out branch of this non-bare repo; allow it.
  git -C "${GITOPS_DIR}" config receive.denyCurrentBranch updateInstead 2>/dev/null || true

  case "${mode}" in
    host)
      # Fast dev loop: run the catalog binary on the host against the platform's
      # GitOps repo + kubeconfig. Rebuild + restart is instant; no image build.
      need go kubectl
      build_web
      log "Running the catalog on the host against ${GITOPS_DIR} (Ctrl-C to stop)"
      log "Open http://localhost:${PORT}/"
      CATALOG_LISTEN=":${PORT}" \
      CATALOG_GITOPS_REPO_URL="file://${GITOPS_DIR}" \
      CATALOG_ARGOCD_ENABLED="true" \
      CATALOG_ARGOCD_NAMESPACE="argocd" \
      CATALOG_ARGOCD_ROOT_APP="nebari-root" \
      CATALOG_DOMAIN="nebari.local" \
        go run "${ROOT}/cmd/catalog"
      ;;
    in-cluster)
      # Faithful: deploy the catalog as a pack via the platform's GitOps repo.
      need docker kind kubectl
      build_image
      log "Loading ${IMAGE} into kind cluster ${KIND_CLUSTER}"
      kind load docker-image "${IMAGE}" --name "${KIND_CLUSTER}"
      log "Registering the catalog Application into ${GITOPS_DIR}/apps/"
      mkdir -p "${GITOPS_DIR}/nebari-catalog-pack" "${GITOPS_DIR}/apps"
      cp -r "${ROOT}/chart/." "${GITOPS_DIR}/nebari-catalog-pack/"
      GITOPS_DIR="${GITOPS_DIR}" envsubst < "${ROOT}/test/integration/catalog-application.yaml" \
        > "${GITOPS_DIR}/apps/nebari-catalog-pack.yaml"
      ( cd "${GITOPS_DIR}" && git add -A && \
        git -c user.email=dev@catalog -c user.name=catalog-dev commit -m "add nebari-catalog-pack" || true )
      chmod -R a+rX "${GITOPS_DIR}"
      kubectl -n argocd annotate application nebari-root argocd.argoproj.io/refresh=normal --overwrite || true
      log "Committed. Watch it sync with: dev/cluster.sh status"
      ;;
    *)
      die "unknown catalog mode '${mode}' (use: host | in-cluster)"
      ;;
  esac
}

cmd_status() {
  need kubectl
  echo "── ArgoCD Applications ──"; kubectl get applications -n argocd 2>/dev/null || true
  echo "── Pods (nebari-system) ──"; kubectl get pods -n nebari-system 2>/dev/null || true
}

case "${1:-}" in
  quick)       cmd_quick ;;
  quick-down)  cmd_quick_down ;;
  up)          cmd_up ;;
  down)        cmd_down ;;
  catalog)     shift; cmd_catalog "${1:-host}" ;;
  status)      cmd_status ;;
  *)
    grep -E '^#( |$)' "$0" | sed -E 's/^# ?//' | sed -n '1,40p'
    exit 1
    ;;
esac
