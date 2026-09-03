import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: ".",
  timeout: 30_000,
  expect: { timeout: 10_000 },
  use: {
    baseURL: process.env.GOSHORT_URL ?? "http://localhost:8099",
    trace: "retain-on-failure",
  },
  reporter: [["list"]],
});
