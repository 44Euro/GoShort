import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";

import { api } from "./api";
import { adminEnabled } from "./role";

type Session = {
  email: string | null;
  checking: boolean;
  signIn: (email: string, password: string) => Promise<void>;
  signOut: () => Promise<void>;
};

const Ctx = createContext<Session | null>(null);

// token อยู่ใน httpOnly cookie อ่าน exp จาก JS ไม่ได้ จึงต้องถามเซิร์ฟเวอร์
// ตอน boot แล้วจัดการ 401 แบบตั้งรับ แทนการเช็ควันหมดอายุล่วงหน้า
export function SessionProvider({ children }: { children: React.ReactNode }) {
  const [email, setEmail] = useState<string | null>(null);
  // instance ที่ไม่มีบทบาท admin ไม่มี session ให้ถาม อย่าเสีย request ทิ้งทุกครั้งที่โหลดหน้า
  const [checking, setChecking] = useState(adminEnabled);

  useEffect(() => {
    if (!adminEnabled) return;

    let alive = true;
    api
      .get<{ email: string }>("/api/admin/me")
      .then((r) => alive && setEmail(r.email))
      .catch(() => alive && setEmail(null))
      .finally(() => alive && setChecking(false));
    return () => {
      alive = false;
    };
  }, []);

  const signIn = useCallback(async (e: string, password: string) => {
    const r = await api.post<{ email: string }>("/api/admin/login", { email: e, password });
    setEmail(r.email);
  }, []);

  const signOut = useCallback(async () => {
    await api.post("/api/admin/logout");
    setEmail(null);
  }, []);

  const value = useMemo(() => ({ email, checking, signIn, signOut }), [email, checking, signIn, signOut]);
  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

export function useSession(): Session {
  const s = useContext(Ctx);
  if (!s) throw new Error("useSession must be used inside SessionProvider");
  return s;
}
