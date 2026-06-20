# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Initial release of `nebari-catalog-pack`.
- Registry discovery: enumerate packs from a Quay/OCI registry (default
  `quay.io/nebari/charts`) with best-effort `pack-metadata.yaml` enrichment.
- GitOps install: generate an ArgoCD `Application` (OCI or git source) and commit
  it into the cluster's GitOps repo, then nudge ArgoCD and poll for health.
- templ/htmx card gallery served by a single Go binary.
- Helm chart packaging the service as a Nebari software pack (gated `NebariApp`,
  scoped RBAC, admin-only auth default).
