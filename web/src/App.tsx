import { lazy, Suspense } from "react";
import { Route, Routes } from "react-router-dom";

import { BootBlank } from "./components/BootBlank";
import { NotFound } from "./pages/NotFound";
import { PublicStats } from "./pages/PublicStats";
import { Shorten } from "./pages/Shorten";
import { adminEnabled } from "./role";
import { SessionProvider } from "./session";

// หน้าสาธารณะไม่ควรดาวน์โหลดโค้ดของหน้าผู้ดูแลติดไปด้วย — นี่เป็นเรื่องขนาด payload
// ไม่ใช่ขอบเขตความปลอดภัย ขอบเขตจริงคือ route ฝั่งเซิร์ฟเวอร์ที่ตอบ 404
const AdminSection = lazy(() => import("./AdminSection"));
const Login = lazy(() => import("./pages/Login").then((m) => ({ default: m.Login })));

export function App() {
  return (
    <SessionProvider>
      <Routes>
        <Route path="/" element={<Shorten />} />
        <Route path="/s" element={<PublicStats />} />
        <Route path="/s/:code" element={<PublicStats />} />

        {adminEnabled && (
          <Route
            path="/login"
            element={
              <Deferred>
                <Login />
              </Deferred>
            }
          />
        )}
        {adminEnabled && (
          <Route
            path="/admin/*"
            element={
              <Deferred>
                <AdminSection />
              </Deferred>
            }
          />
        )}

        {/* /:code ที่ไม่มีจริงถูกเสิร์ฟจาก Go พร้อม status 404 แล้วมาลงที่นี่ */}
        <Route path="*" element={<NotFound />} />
      </Routes>
    </SessionProvider>
  );
}

function Deferred({ children }: { children: React.ReactNode }) {
  return <Suspense fallback={<BootBlank />}>{children}</Suspense>;
}
