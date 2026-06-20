// Captures the catalog UI for the README/docs. Run with: npm run screenshots
//
// The screens are deterministic: the server runs in demo mode (fixed pack list)
// and dry-run (Preview renders the Application manifest without committing).
const { test, expect } = require("@playwright/test");
const fs = require("fs");
const path = require("path");

const DOCS = path.resolve(__dirname, "../../docs/screenshots");

test.beforeAll(() => fs.mkdirSync(DOCS, { recursive: true }));

async function openGallery(page) {
  await page.goto("/");
  await expect(page.locator(".card").first()).toBeVisible();
  // Let card fonts/gradients settle before capture.
  await page.waitForTimeout(400);
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

  const pack = "nebari-lgtm-pack";
  const card = page.locator(`.card[data-pack="${pack}"]`);
  await card.scrollIntoViewIfNeeded();

  // The action button is overlaid on the card (revealed on hover); click it,
  // then move the mouse away so the result reads cleanly in the shot.
  await card.getByRole("button", { name: "Preview" }).click();
  await expect(page.locator(`#result-${pack} .result`)).toBeVisible();

  // Expand the rendered Application manifest.
  await page.locator(`#result-${pack} details summary`).click();
  await expect(page.locator(`#result-${pack} pre.manifest`)).toBeVisible();
  await page.mouse.move(0, 0);
  await page.waitForTimeout(300);

  await card.screenshot({ path: path.join(DOCS, "install-preview.png") });
});
