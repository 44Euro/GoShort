import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// proxy ให้ browser เห็นเป็น same-origin เหมือนตอน prod ไม่งั้น httpOnly cookie
// ข้าม port ไม่ได้ และเราจะไปเจอปัญหา CORS ที่ prod ไม่มีจริง
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      "/api": "http://localhost:8080",
      "/health": "http://localhost:8080",
      "/metrics": "http://localhost:8080",
    },
  },
  build: { outDir: "dist", emptyOutDir: true },
});
