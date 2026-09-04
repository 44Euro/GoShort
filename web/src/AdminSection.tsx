import { Navigate, Route, Routes, useNavigate } from "react-router-dom";

import { BootBlank } from "./components/BootBlank";
import { AdminNav } from "./components/Nav";
import { Analytics } from "./pages/Analytics";
import { Dashboard } from "./pages/Dashboard";
import { Links } from "./pages/Links";
import { NotFound } from "./pages/NotFound";
import { Overview } from "./pages/Overview";
import { useSession } from "./session";

export default function AdminSection() {
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
      {/* mount อยู่ใต้ /admin/* path ที่นี่จึงสัมพัทธ์กับ /admin */}
      <Routes>
        <Route path="/" element={<Dashboard />} />
        <Route path="analytics" element={<Overview />} />
        <Route path="links" element={<Links />} />
        <Route path="links/:code" element={<Analytics />} />
        <Route path="*" element={<NotFound />} />
      </Routes>
    </>
  );
}
