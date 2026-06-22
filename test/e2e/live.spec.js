// Captures the catalog UI against a LIVE deployment and drives a real install.
// Run via: CATALOG_URL=... TEST_PACK=... npm run screenshots:live
//
// Used by .github/workflows/integration.yml once the action-nebari-sandbox
// platform is up and the catalog is running on the runner pointed at the
// sandbox's real file:// GitOps repo + ArgoCD. The screenshots therefore show
// the real registry gallery and a genuine install result (committed to git,
// nudged into ArgoCD) — not the deterministic demo fixtures.
//
// The pack to install comes from TEST_PACK; if that card is not present in the
// live listing we fall back to the first installable card. The chosen pack name
// is written to installed-pack.txt so the workflow knows which ArgoCD
// Application to assert on afterward.
const { test, expect } = require("@playwright/test");
const fs = require("fs");
const path = require("path");

const OUT = path.resolve(__dirname, "../../docs/screenshots/live");
const WANT = process.env.TEST_PACK || "nebari-lgtm-pack";

test.beforeAll(() => fs.mkdirSync(OUT, { recursive: true }));

async function openGallery(page) {
  await page.goto("/");
  await expect(page.locator(".card").first()).toBeVisible();
  await page.waitForTimeout(400);
}

test("live gallery (light)", async ({ page }) => {
  await page.emulateMedia({ colorScheme: "light" });
  await openGallery(page);
  await page.screenshot({ path: path.join(OUT, "gallery-light.png"), fullPage: true });
});

test("live gallery (dark)", async ({ page }) => {
  await page.emulateMedia({ colorScheme: "dark" });
  await openGallery(page);
  await page.screenshot({ path: path.join(OUT, "gallery-dark.png"), fullPage: true });
});

test("live install", async ({ page }) => {
  await page.emulateMedia({ colorScheme: "light" });
  await openGallery(page);

  // Prefer the requested pack; fall back to the first card if it is not listed.
  let pack = WANT;
  if ((await page.locator(`.card[data-pack="${WANT}"]`).count()) === 0) {
    pack = await page.locator(".card").first().getAttribute("data-pack");
  }
  fs.writeFileSync(path.join(__dirname, "installed-pack.txt"), pack ?? "");

  const card = page.locator(`.card[data-pack="${pack}"]`);
  await card.scrollIntoViewIfNeeded();

  // "Install" in a live (GitOps-configured) deployment; "Preview" in dry-run —
  // match either so the spec is runnable locally too.
  await card.getByRole("button", { name: /Install|Preview/ }).click();

  // The real install commits + nudges + polls ArgoCD, so the result can take a
  // while; the config's expect timeout covers it.
  await expect(page.locator(`#result-${pack} .result`)).toBeVisible();

  await page.mouse.move(0, 0);
  await page.waitForTimeout(300);
  await card.screenshot({ path: path.join(OUT, "install-result.png") });
});
