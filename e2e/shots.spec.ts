import { test } from "@playwright/test";

const shots = "shots";

test("capture every screen", async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 1000 });

  await page.goto("/");
  await page.waitForTimeout(600);
  await page.screenshot({ path: `${shots}/01-shorten.png`, fullPage: true });

  await page.goto("/s/gopher");
  await page.waitForTimeout(600);
  await page.screenshot({ path: `${shots}/02-public-stats.png`, fullPage: true });

  await page.goto("/nosuchcode");
  await page.waitForTimeout(600);
  await page.screenshot({ path: `${shots}/03-404.png` });

  await page.goto("/login");
  await page.waitForTimeout(400);
  await page.screenshot({ path: `${shots}/04-login.png` });

  await page.getByLabel("Password").fill("goshort-demo");
  await page.getByRole("button", { name: "Sign in" }).click();
  await page.waitForURL("**/admin");
  await page.waitForTimeout(1400);
  await page.screenshot({ path: `${shots}/05-dashboard.png`, fullPage: true });

  await page.goto("/admin/links");
  await page.waitForTimeout(700);
  await page.screenshot({ path: `${shots}/06-links.png`, fullPage: true });

  await page.goto("/admin/links/gopher");
  await page.waitForTimeout(700);
  await page.screenshot({ path: `${shots}/07-analytics.png`, fullPage: true });

  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/");
  await page.waitForTimeout(600);
  await page.screenshot({ path: `${shots}/08-mobile-shorten.png`, fullPage: true });

  await page.goto("/admin/links");
  await page.waitForTimeout(700);
  await page.screenshot({ path: `${shots}/09-mobile-links.png`, fullPage: true });
});
