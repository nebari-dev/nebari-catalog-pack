// Captures the catalog SPA for the README/docs. Run with: npm run screenshots
//
// Deterministic: the Go binary runs in demo mode (fixed pack list) + dry-run,
// so the grid renders offline data and Preview renders the Application manifest
// without committing. The theme follows the emulated color scheme.
const { test, expect } = require("@playwright/test");
const fs = require("fs");
const path = require("path");

const DOCS = path.resolve(__dirname, "../../docs/screenshots");

test.beforeAll(() => fs.mkdirSync(DOCS, { recursive: true }));

async function openGallery(page) {
  await page.goto("/");
  await expect(page.locator(".card").first()).toBeVisible();
  await page.waitForTimeout(500); // let fonts/gradients settle
}

test("gallery (light)", async ({ page }) => {
  await page.emulateMedia({ colorScheme: "light" });
  await openGallery(page);
  await page.screenshot({ path: path.join(DOCS, "gallery-light.png"), fullPage: true });
});

test("gallery (dark)", async ({ page }) => {
  await page.emulateMedia({ colorScheme: "dark" });
  await openGallery(page);
  await page.screenshot({ path: path.join(DOCS, "gallery-dark.png"), fullPage: true });
});

test("install preview", async ({ page }) => {
  await page.emulateMedia({ colorScheme: "light" });
  await openGallery(page);

  const pack = "nebari-lgtm-pack"; // present in demo fixtures, not installed
  const card = page.locator(`.card[data-pack="${pack}"]`);
  await card.scrollIntoViewIfNeeded();
  await card.getByRole("button", { name: "Preview" }).click();
  await expect(card.locator(".result")).toBeVisible();
  await card.locator(".manifest summary").click();
  await expect(card.locator(".manifest pre")).toBeVisible();
  await page.waitForTimeout(300);
  await card.screenshot({ path: path.join(DOCS, "install-preview.png") });
});
