// Playwright config for capturing the catalog UI against a LIVE deployment.
//
// Unlike playwright.config.js (which boots the server itself in demo + dry-run
// mode for deterministic README screenshots), this config attaches to an
// already-running catalog server — pointed at a real registry and a real
// GitOps repo — via CATALOG_URL. It is driven by .github/workflows/
// integration.yml after the action-nebari-sandbox platform is up, so the
// screenshots reflect an actual deployment installing a real pack.
//
// No webServer block: the integration workflow starts the catalog process.
// Output lands in docs/screenshots/live/ (see live.spec.js).
const { defineConfig } = require("@playwright/test");

const BASE = process.env.CATALOG_URL || "http://localhost:8080";

module.exports = defineConfig({
  testDir: ".",
  testMatch: /live\.spec\.js$/,
  fullyParallel: false,
  workers: 1,
  // A real install commits to git, nudges ArgoCD, and polls the child
  // Application — the /install request can block for minutes. Give the suite
  // headroom so the result fragment (with live Health/Sync) renders.
  timeout: 420_000,
  expect: { timeout: 300_000 },
  use: {
    baseURL: BASE,
    viewport: { width: 1440, height: 900 },
    deviceScaleFactor: 2,
    headless: true,
  },
});
