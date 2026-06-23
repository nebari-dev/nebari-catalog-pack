// Captures the catalog SPA against a LIVE deployment and triggers a real
// install. Run via: CATALOG_URL=... TEST_PACK=... npm run screenshots:live
//
// Used by .github/workflows/integration.yml once the action-nebari-sandbox
// platform is up and the catalog runs on the runner pointed at the sandbox's
// real file:// GitOps repo + ArgoCD. The install is triggered through the JSON
// API (robust, no UI timing), then the page is reloaded so the screenshot shows
// the resulting cluster state. The chosen pack is written to installed-pack.txt
// so the workflow knows which ArgoCD Application to assert on.
const { test, expect } = require("@playwright/test");
const fs = require("fs");
const path = require("path");

const OUT = path.resolve(__dirname, "../../docs/screenshots/live");
const WANT = process.env.TEST_PACK || "nebari-lgtm-pack";

test.beforeAll(() => fs.mkdirSync(OUT, { recursive: true }));

async function openGallery(page) {
  await page.goto("/");
  await expect(page.locator(".card").first()).toBeVisible();
  await page.waitForTimeout(500);
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

test("live install", async ({ page, request }) => {
  await page.emulateMedia({ colorScheme: "light" });
  await openGallery(page);

  // Prefer the requested pack; fall back to the first card if it is not listed.
  let pack = WANT;
  if ((await page.locator(`.card[data-pack="${WANT}"]`).count()) === 0) {
    pack = await page.locator(".card").first().getAttribute("data-pack");
  }
  fs.writeFileSync(path.join(__dirname, "installed-pack.txt"), pack ?? "");

  // Trigger a real (non-dry-run) install through the API the SPA uses.
  const res = await request.post("api/install", {
    headers: { "Content-Type": "application/json" },
    data: { pack, version: "", dryRun: false },
  });
  expect(res.ok()).toBeTruthy();

  // Reload so the grid reflects the new cluster state, then capture it.
  await page.reload();
  await expect(page.locator(".card").first()).toBeVisible();
  await page.waitForTimeout(800);
  const card = page.locator(`.card[data-pack="${pack}"]`);
  if (await card.count()) await card.scrollIntoViewIfNeeded();
  await page.screenshot({ path: path.join(OUT, "install-result.png"), fullPage: true });
});
