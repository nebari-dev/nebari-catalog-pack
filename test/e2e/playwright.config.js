// Playwright config for the screenshot harness.
//
// It builds and runs the Go binary in demo + dry-run mode on a fixed port, so
// the gallery renders deterministic offline data (registry.Fixtures) and the
// install button performs a no-op "Preview" that renders the Application
// manifest — no cluster, no network, no git writes.
const { defineConfig } = require("@playwright/test");

const PORT = process.env.PORT || "8757";
const BASE = `http://localhost:${PORT}`;

module.exports = defineConfig({
  testDir: ".",
  fullyParallel: false,
  workers: 1,
  timeout: 60_000,
  expect: { timeout: 10_000 },
  use: {
    baseURL: BASE,
    viewport: { width: 1440, height: 900 },
    deviceScaleFactor: 2,
    headless: true,
  },
  webServer: {
    // Run from test/e2e/, so the package is two levels up.
    command: "go run ../../cmd/catalog",
    url: `${BASE}/healthz`,
    timeout: 120_000,
    reuseExistingServer: !process.env.CI,
    env: {
      CATALOG_LISTEN: `:${PORT}`,
      CATALOG_DEMO: "true",
      CATALOG_DRY_RUN: "true",
      // A dummy repo makes installs "enabled" so Preview buttons render; dry-run
      // means nothing is ever committed to it.
      CATALOG_GITOPS_REPO_URL: "file:///tmp/catalog-demo-gitops",
      CATALOG_ARGOCD_ENABLED: "false",
      CATALOG_DOMAIN: "nebari.example.com",
    },
  },
});
