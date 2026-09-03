import { Navigate, Outlet, Route, Routes, useNavigate } from "react-router-dom";

import { AdminNav } from "./components/Nav";
import { Analytics } from "./pages/Analytics";
import { Dashboard } from "./pages/Dashboard";
import { Links } from "./pages/Links";
import { BootBlank, Login } from "./pages/Login";
import { NotFound } from "./pages/NotFound";
import { PublicStats } from "./pages/PublicStats";
import { Shorten } from "./pages/Shorten";
import { SessionProvider, useSession } from "./session";

export function App() {
  return (
    <SessionProvider>
      <Routes>
        <Route path="/" element={<Shorten />} />
        <Route path="/s" element={<PublicStats />} />
        <Route path="/s/:code" element={<PublicStats />} />
        <Route path="/login" element={<Login />} />

        <Route element={<AdminLayout />}>
          <Route path="/admin" element={<Dashboard />} />
          <Route path="/admin/links" element={<Links />} />
          <Route path="/admin/links/:code" element={<Analytics />} />
        </Route>

        {/* /:code ที่ไม่มีจริงถูกเสิร์ฟจาก Go พร้อม status 404 แล้วมาลงที่นี่ */}
        <Route path="*" element={<NotFound />} />
      </Routes>
    </SessionProvider>
  );
}

function AdminLayout() {
  const { email, checking, signOut } = useSession();
  const nav = useNavigate();

  // ระหว่างถามเซิร์ฟเวอร์ว่ายัง login อยู่ไหม ต้องไม่กระพริบหน้า login ให้เห็นก่อน
  if (checking) return <BootBlank />;
  if (!email) return <Navigate to="/login" replace />;

  return (
    <>
      <AdminNav
        email={email}
        onSignOut={async () => {
          // session ที่หมดอายุไปแล้วทำให้ logout ล้มได้ แต่ผู้ใช้ก็ยังต้องออกจากระบบ
          try {
            await signOut();
          } finally {
            nav("/", { replace: true });
          }
        }}
      />
      <Outlet />
    </>
  );
}
