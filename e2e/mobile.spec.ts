import { expect, test } from "@playwright/test";

const screens = [
  { path: "/", name: "01-shorten" },
  { path: "/s/demo1", name: "02-public-stats" },
  { path: "/nosuchcode", name: "03-404" },
  { path: "/login", name: "04-login" },
];

const adminScreens = [
  { path: "/admin", name: "05-dashboard" },
  { path: "/admin/links", name: "06-links" },
  { path: "/admin/links/demo1", name: "07-analytics" },
];

test("every screen is usable at 360px", async ({ page }) => {
  await page.setViewportSize({ width: 360, height: 800 });

  const overflow = async (name: string) => {
    const wide = await page.evaluate(
      () => document.documentElement.scrollWidth > document.documentElement.clientWidth + 1,
    );
    expect(wide, `${name} makes the page scroll sideways at 360px`).toBe(false);
  };

  for (const s of screens) {
    await page.goto(s.path);
    await page.waitForTimeout(500);
    await overflow(s.name);
    await page.screenshot({ path: `mobile/${s.name}.png`, fullPage: true });
  }

  await page.goto("/login");
  await page.getByLabel("Password").fill("goshort-demo");
  await page.getByRole("button", { name: "Sign in" }).click();
  await page.waitForURL("**/admin");

  for (const s of adminScreens) {
    await page.goto(s.path);
    await page.waitForTimeout(900);
    await overflow(s.name);
    await page.screenshot({ path: `mobile/${s.name}.png`, fullPage: true });
  }
});
