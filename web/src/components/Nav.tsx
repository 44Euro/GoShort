import { Link, useLocation } from "react-router-dom";

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
      <Link to="/login">Administration</Link>
    </div>
  );
}

export function AdminNav({ email, onSignOut }: { email: string; onSignOut: () => void }) {
  const { pathname } = useLocation();
  const isLink = (p: string) => (pathname === p ? "nav-link-active" : undefined);

  return (
    <div className="nav">
      <Brand to="/admin" />
      <Link to="/admin" className={isLink("/admin")}>Operations</Link>
      <Link to="/admin/links" className={isLink("/admin/links")}>Links</Link>
      <Link
        to="/admin/links"
        className={pathname.startsWith("/admin/links/") ? "nav-link-active" : undefined}
      >
        Analytics
      </Link>
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
