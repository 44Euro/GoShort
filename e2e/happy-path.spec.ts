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
  // กรองก่อน ไม่ใช่หวังว่าลิงก์ใหม่จะอยู่หน้าแรก — ตารางแบ่งหน้าละ 8 และฐานข้อมูล
  // ที่ใช้จริงมีลิงก์อื่นปนอยู่เสมอ
  await page.getByLabel("Filter links").fill(code);
  const row = page.getByRole("row").filter({ hasText: `/${code}` });
  await expect(row).toBeVisible();

  await row.getByRole("button", { name: "Delete" }).click();
  const confirm = page.getByRole("dialog");
  await expect(confirm.getByText(`Delete /${code}?`)).toBeVisible();
  await confirm.getByRole("button", { name: "Delete the link" }).click();
  await expect(row).toHaveCount(0);

  const gone = await request.get(`${baseURL}/${code}`, { maxRedirects: 0 });
  expect(gone.status(), "deletion must not wait for the cache TTL").toBe(404);
});

test("an unknown code answers 404 rather than pretending the page exists", async ({ request, baseURL }) => {
  const res = await request.get(`${baseURL}/definitelynotacode`, { maxRedirects: 0 });
  expect(res.status()).toBe(404);
});

// ก่อนแก้: เปลี่ยนลิงก์แล้วตัวเลขของลิงก์เดิมค้างอยู่ใต้หัวข้อของลิงก์ใหม่
test("switching between links never shows the previous link's numbers", async ({ page, request, baseURL }) => {
  // สร้างข้อมูลของตัวเองเสมอ ฐานข้อมูลที่รันจริงมีลิงก์จาก benchmark ปนอยู่
  const make = async (clicks: number) => {
    const res = await request.post(`${baseURL}/api/links`, {
      data: { long_url: `https://go.dev/switch/${Math.random()}` },
    });
    const { code } = (await res.json()) as { code: string };
    for (let i = 0; i < clicks; i++) {
      await request.get(`${baseURL}/${code}`, { maxRedirects: 0 });
    }

    // worker เขียนเป็น batch ทุกสองวินาที รอที่ API ก่อน — หน้า analytics ไม่ poll
    // ตัวเอง การไปรอที่ DOM จึงรอไปก็เท่านั้น
    await expect
      .poll(async () => {
        const r = await request.get(`${baseURL}/api/links/${code}/stats`);
        return ((await r.json()) as { clicks: number }).clicks;
      }, { timeout: 15_000 })
      .toBe(clicks);

    return code;
  };

  const busyCode = await make(5);
  const quietCode = await make(1);

  await page.goto("/login");
  await page.getByLabel("Password").fill(process.env.ADMIN_PASSWORD ?? "goshort-demo");
  await page.getByRole("button", { name: "Sign in" }).click();
  await page.waitForURL("**/admin");

  const total = async (code: string) => {
    await page.goto(`/admin/links/${code}`);
    await expect(page.getByRole("heading", { name: `/${code}` })).toBeVisible();
    return page.locator(".tile-value").first().innerText();
  };

  const busy = await total(busyCode);
  const quiet = await total(quietCode);

  expect(quiet, "the second link must not inherit the first link's click count").not.toBe(busy);
  expect(busy).toBe("5");
  expect(quiet).toBe("1");
});

test("an error clears as soon as the field is corrected", async ({ page }) => {
  await page.goto("/");
  await page.getByLabel("Long URL").fill("not-a-url");
  await page.getByRole("button", { name: "Shorten" }).click();

  const error = page.locator(".field-error").first();
  await expect(error).toBeVisible();

  await page.getByLabel("Long URL").fill("https://go.dev/");
  await expect(error).toHaveCount(0);
});
