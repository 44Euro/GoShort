import { Link, useLocation } from "react-router-dom";

import { adminEnabled } from "../role";

export function Brand({ to = "/" }: { to?: string }) {
  return (
    <Link to={to} className="nav-brand" style={{ textDecoration: "none", color: "inherit" }}>
      GoShort<span>.</span>
    </Link>
  );
}

export function PublicNav() {
  return (
    <div className="nav">
      <Brand />
      <span className="nav-spacer" />
      <Link to="/s">Look up a code</Link>
      {adminEnabled && <Link to="/login">Sign in</Link>}
    </div>
  );
}

export function AdminNav({ email, onSignOut }: { email: string; onSignOut: () => void }) {
  const { pathname } = useLocation();
  // หน้าสถิติรายลิงก์อยู่ใต้ /admin/links/:code และเข้าถึงจากตาราง ปุ่ม Links
  // จึงต้องติดอยู่ตอนอยู่หน้านั้นด้วย
  const active = (p: string) =>
    pathname === p || (p === "/admin/links" && pathname.startsWith("/admin/links/"))
      ? "nav-link-active"
      : undefined;

  return (
    <div className="nav">
      <Brand to="/admin" />
      <Link to="/admin" className={active("/admin")}>Operations</Link>
      <Link to="/admin/links" className={active("/admin/links")}>Links</Link>
      <Link to="/admin/analytics" className={active("/admin/analytics")}>Analytics</Link>
      <span className="nav-spacer" />
      <span className="nav-email">{email}</span>
      <a
        href="#"
        onClick={(e) => {
          e.preventDefault();
          onSignOut();
        }}
      >
        Sign out
      </a>
    </div>
  );
}
