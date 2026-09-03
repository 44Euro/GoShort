import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";

import { ApiError, api, type LinkStats } from "../api";
import { DailyBars, ReferrerBars } from "../components/Charts";
import { Brand } from "../components/Nav";
import { NotFound } from "./NotFound";

export function PublicStats() {
  const { code } = useParams();
  const [data, setData] = useState<LinkStats | null>(null);
  const [status, setStatus] = useState<"loading" | "ready" | "missing" | "error">("loading");

  useEffect(() => {
    if (!code) return;
    let alive = true;
    setStatus("loading");
    api
      .get<LinkStats>(`/api/links/${encodeURIComponent(code)}/stats`)
      .then((d) => {
        if (!alive) return;
        setData(d);
        setStatus("ready");
      })
      .catch((err) => {
        if (!alive) return;
        setStatus(err instanceof ApiError && err.status === 404 ? "missing" : "error");
      });
    return () => {
      alive = false;
    };
  }, [code]);

  if (!code) return <Lookup />;
  if (status === "missing") return <NotFound />;

  return (
    <div className="rise">
      <div className="nav">
        <Brand />
        <span className="nav-spacer" />
        <Link to="/">Shorten a link</Link>
        <Link to="/login">Administration</Link>
      </div>

      <div className="stats-page">
        <div className="eyebrow" style={{ color: "var(--color-neutral-600)", marginBottom: 22 }}>
          Public statistics · no account required
        </div>
        <h1 className="stats-code">{window.location.host}/{code}</h1>
        <div
          className="mono muted"
          style={{ fontSize: 13, wordBreak: "break-all", marginBottom: 44 }}
        >
          {data ? `→ ${data.long_url}` : " "}
        </div>

        <div className="stats-band">
          <div className="stats-big">
            {status === "loading" ? "—" : (data?.clicks ?? 0).toLocaleString()}
          </div>
          <div
            className="mono muted"
            style={{ fontSize: 10.5, letterSpacing: "0.22em", textTransform: "uppercase", marginTop: 16 }}
          >
            Total clicks
          </div>
        </div>

        {status === "error" && (
          <div className="form-error" style={{ textAlign: "left" }}>
            Could not reach the API for this link.
          </div>
        )}

        <div style={{ marginTop: 34 }}>
          <DailyBars points={data?.series ?? []} height={120} />
        </div>
        <div
          className="mono muted"
          style={{ fontSize: 10, letterSpacing: "0.16em", textTransform: "uppercase", marginTop: 12 }}
        >
          Last fourteen days
        </div>

        <div style={{ marginTop: 54, textAlign: "left" }}>
          <div className="section-rule">Where the clicks came from</div>
          <ReferrerBars items={data?.referrers ?? []} />
        </div>
      </div>
    </div>
  );
}

function Lookup() {
  const [code, setCode] = useState("");

  return (
    <div className="rise">
      <div className="nav">
        <Brand />
        <span className="nav-spacer" />
        <Link to="/">Shorten a link</Link>
        <Link to="/login">Administration</Link>
      </div>

      <div className="stats-page">
        <div className="eyebrow" style={{ color: "var(--color-neutral-600)", marginBottom: 22 }}>
          Public statistics · no account required
        </div>
        <h1 style={{ fontSize: 46, fontWeight: 400, letterSpacing: "-0.025em", margin: "0 0 34px" }}>
          Look up a code
        </h1>
        <form
          onSubmit={(e) => {
            e.preventDefault();
            if (code.trim()) window.location.assign(`/s/${encodeURIComponent(code.trim())}`);
          }}
          style={{ display: "flex", gap: 12, alignItems: "flex-end", textAlign: "left" }}
        >
          <div className="field" style={{ flex: 1 }}>
            <label className="label-mono" htmlFor="lookup">Short code</label>
            <input
              id="lookup"
              className="input input-rule"
              placeholder="gopher"
              value={code}
              onChange={(e) => setCode(e.target.value)}
            />
          </div>
          <button className="btn btn-primary" style={{ minHeight: 42 }}>Look up</button>
        </form>
      </div>
    </div>
  );
}
