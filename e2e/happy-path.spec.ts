import { expect, test } from "@playwright/test";

const admin = { email: "admin@goshort.dev", password: process.env.ADMIN_PASSWORD ?? "goshort-demo" };

// เส้นเดียวผ่าน stack ที่ประกอบเสร็จแล้ว เจตนาคือพิสูจน์ว่าชิ้นส่วนต่อกันติด
// ไม่ใช่ทดสอบ logic ซ้ำกับเทสต์ฝั่ง Go
test("shorten, redirect, see the click, then delete it", async ({ page, request, baseURL }) => {
  const destination = "https://go.dev/blog/pipelines";

  await page.goto("/");
  await page.getByLabel("Long URL").fill(destination);
  await page.getByRole("button", { name: "Shorten" }).click();

  await expect(page.getByText("201 Created")).toBeVisible();
  const shown = await page.locator(".result-url").innerText();
  const code = shown.split("/").pop()!;
  expect(code).toHaveLength(7);

  // redirect จริง ไม่ตาม redirect เพื่อดู 302 กับ Location ตรง ๆ
  const hop = await request.get(`${baseURL}/${code}`, { maxRedirects: 0 });
  expect(hop.status()).toBe(302);
  expect(hop.headers()["location"]).toBe(destination);

  await expect(async () => {
    await page.goto(`/s/${code}`);
    await expect(page.locator(".stats-big")).toHaveText("1");
  }).toPass({ timeout: 15_000 });

  await page.goto("/login");
  await page.getByLabel("Email").fill(admin.email);
  await page.getByLabel("Password").fill(admin.password);
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page).toHaveURL(/\/admin$/);

  // session ต้องอยู่ข้าม refresh เพราะ token อยู่ใน httpOnly cookie
  await page.reload();
  await expect(page.getByText(admin.email)).toBeVisible();

  await page.goto("/admin/links");
  const row = page.getByRole("row").filter({ hasText: `/${code}` });
  await expect(row).toBeVisible();

  page.once("dialog", (d) => d.accept());
  await row.getByRole("button", { name: "Delete" }).click();
  await expect(row).toHaveCount(0);

  const gone = await request.get(`${baseURL}/${code}`, { maxRedirects: 0 });
  expect(gone.status(), "deletion must not wait for the cache TTL").toBe(404);
});

test("an unknown code answers 404 rather than pretending the page exists", async ({ request, baseURL }) => {
  const res = await request.get(`${baseURL}/definitelynotacode`, { maxRedirects: 0 });
  expect(res.status()).toBe(404);
});
