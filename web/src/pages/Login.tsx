import { useState } from "react";
import { Link, Navigate, useNavigate } from "react-router-dom";

import { useSession } from "../session";

export function Login() {
  const { email: signedIn, checking, signIn } = useSession();
  const nav = useNavigate();
  const [email, setEmail] = useState("admin@goshort.dev");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  if (checking) return <BootBlank />;
  if (signedIn) return <Navigate to="/admin" replace />;

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await signIn(email, password);
      nav("/admin", { replace: true });
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="login">
      <div className="dark-panel login-aside">
        <span className="sweep" />
        <div style={{ fontFamily: "var(--font-heading)", fontSize: 22, letterSpacing: "-0.02em" }}>
          GoShort<span style={{ color: "var(--color-ink-accent)" }}>.</span>
        </div>
        <div>
          <div
            className="ghost"
            style={{ position: "static", fontSize: 200, lineHeight: 0.8, marginBottom: -20 }}
            aria-hidden="true"
          >
            302
          </div>
          <p
            style={{
              maxWidth: 380,
              fontSize: 14,
              lineHeight: 1.7,
              color: "var(--color-ink-muted)",
              textAlign: "justify",
              margin: "26px 0 0",
            }}
          >
            A single seeded administrator. No registration, no password reset, no second account — the
            credential is created once by{" "}
            <span className="mono" style={{ fontSize: 12.5, color: "var(--color-ink-accent)" }}>cmd/seed</span> and checked
            against a bcrypt hash.
          </p>
        </div>
        <div
          className="mono"
          style={{ fontSize: 10.5, letterSpacing: "0.14em", textTransform: "uppercase", color: "var(--color-ink-faint)" }}
        >
          JWT · HS256 · 24h expiry
        </div>
      </div>

      <div className="login-form-wrap">
        <form className="login-form" onSubmit={submit} noValidate>
          <div className="eyebrow" style={{ marginBottom: 14 }}>POST /api/admin/login</div>
          <h1>Administration</h1>

          <div className="field" style={{ marginBottom: 22 }}>
            <label className="label-mono" htmlFor="email">Email</label>
            <input
              id="email"
              className="input input-rule"
              autoComplete="username"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
            />
          </div>
          <div className="field" style={{ marginBottom: 32 }}>
            <label className="label-mono" htmlFor="password">Password</label>
            <input
              id="password"
              className="input input-rule"
              type="password"
              autoComplete="current-password"
              placeholder="set by cmd/seed"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
          </div>

          <button className="btn btn-primary btn-block" style={{ minHeight: 44, fontSize: 14 }} disabled={busy}>
            {busy ? "Signing in…" : "Sign in"}
          </button>

          {error && <div className="form-error">{error}</div>}

          <div
            className="mono muted"
            style={{ marginTop: 26, display: "flex", alignItems: "center", gap: 10, fontSize: 10.5 }}
          >
            <span className="pulse-dot" />
            <span>Single seeded administrator</span>
          </div>
          <div className="hr" style={{ margin: "26px 0" }} />
          <Link to="/" style={{ fontSize: 13, textDecoration: "none" }}>
            ← Back to the public shortener
          </Link>
        </form>
      </div>
    </div>
  );
}

export function BootBlank() {
  return <div style={{ minHeight: "100vh" }} aria-busy="true" />;
}
