import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";

import { ApiError, api, type Analytics as Data } from "../api";
import { DailyBars, Empty, ReferrerBars } from "../components/Charts";
import { NotFound } from "./NotFound";

function countdown(seconds: number): string {
  if (seconds <= 0) return "expired";
  const m = Math.floor(seconds / 60);
  const s = seconds % 60;
  return `${m}m ${String(s).padStart(2, "0")}s`;
}

export function Analytics() {
  const { code } = useParams();
  const [data, setData] = useState<Data | null>(null);
  const [state, setState] = useState<"loading" | "ready" | "missing" | "error">("loading");
  const [ttl, setTtl] = useState(0);
  const [copied, setCopied] = useState(false);
  const [busy, setBusy] = useState(false);

  // ต้องล้าง data ทิ้งตอนเปลี่ยน code ไม่งั้นตัวเลขของลิงก์เดิมจะค้างอยู่ใต้หัวข้อ
  // ของลิงก์ใหม่ และคำตอบที่มาช้ากว่าการเปลี่ยนหน้าต้องถูกทิ้ง ไม่ใช่เขียนทับ
  useEffect(() => {
    if (!code) return;
    let alive = true;

    setData(null);
    setState("loading");

    api
      .get<Data>(`/api/admin/links/${encodeURIComponent(code)}/analytics`)
      .then((d) => {
        if (!alive) return;
        setData(d);
        setTtl(d.cache.ttl_seconds);
        setState("ready");
      })
      .catch((err) => {
        if (!alive) return;
        setState(err instanceof ApiError && err.status === 404 ? "missing" : "error");
      });

    return () => {
      alive = false;
    };
  }, [code]);

  useEffect(() => {
    if (!data?.cache.warm) return;
    const id = setInterval(() => setTtl((t) => Math.max(0, t - 1)), 1000);
    return () => clearInterval(id);
  }, [data?.cache.warm]);

  if (state === "missing") return <NotFound />;
  if (!data) {
    return (
      <div className="page-body">
        {state === "error" ? <div className="form-error">Could not load this link.</div> : <Empty label="loading" />}
      </div>
    );
  }

  const shortUrl = `${window.location.origin}/${data.code}`;

  const invalidate = async () => {
    setBusy(true);
    try {
      await api.post(`/api/admin/links/${encodeURIComponent(data.code)}/invalidate-cache`);
      setData({ ...data, cache: { warm: false, ttl_seconds: 0 } });
      setTtl(0);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="rise">
      <div style={{ padding: "22px 40px 0" }}>
        <Link to="/admin/links" style={{ fontSize: 13, textDecoration: "none" }}>← The register</Link>
      </div>

      <div className="page-head" style={{ paddingTop: 18 }}>
        <div style={{ minWidth: 0 }}>
          <h1 className="mono" style={{ fontSize: 36, fontWeight: 500, letterSpacing: "-0.035em", color: "var(--color-accent-700)" }}>
            /{data.code}
          </h1>
          <div className="mono muted" style={{ fontSize: 13, wordBreak: "break-all", marginTop: 8 }}>
            → {data.long_url}
          </div>
        </div>
        <div style={{ display: "flex", gap: 10, flexWrap: "wrap" }}>
          <button
            className="btn btn-secondary"
            onClick={async () => {
              await navigator.clipboard.writeText(shortUrl);
              setCopied(true);
              setTimeout(() => setCopied(false), 1600);
            }}
          >
            {copied ? "Copied" : "Copy short URL"}
          </button>
          <button className="btn btn-primary" onClick={invalidate} disabled={busy || !data.cache.warm}>
            {busy ? "Invalidating…" : "Invalidate cache"}
          </button>
        </div>
      </div>

      <div className="tile-row" style={{ marginTop: 30 }}>
        <div className="tile">
          <div className="tile-label">Total clicks</div>
          <div className="tile-value">{data.clicks.toLocaleString()}</div>
        </div>
        <div className="tile">
          <div className="tile-label">Unique IP hashes</div>
          <div className="tile-value">{data.unique.toLocaleString()}</div>
        </div>
        <div className="tile">
          <div className="tile-label">Mean per day</div>
          <div className="tile-value">{data.per_day.toFixed(1)}</div>
        </div>
        <div className="tile">
          <div className="tile-label">Cache state</div>
          <div className="tile-value is-word" style={{ display: "flex", alignItems: "center", gap: 10 }}>
            {data.cache.warm && <span className="pulse-dot" />}
            {data.cache.warm ? "Warm" : "Cold"}
          </div>
          <div className="mono muted" style={{ fontSize: 11, marginTop: 8 }}>
            {data.cache.warm ? `TTL expires in ${countdown(ttl)}` : "next hit reads Postgres"}
          </div>
        </div>
      </div>

      <div className="panel">
        <div className="panel-head">
          <h2>Daily click series</h2>
          <span className="mono muted" style={{ fontSize: 11 }}>14 days</span>
        </div>
        <DailyBars points={data.series} />
      </div>

      <div className="two-col">
        <div>
          <h2 style={{ margin: "0 0 8px" }}>Referrers</h2>
          <div className="section-rule" style={{ marginBottom: 4 }} />
          <ReferrerBars items={data.referrers} />
        </div>

        <div>
          <h2 style={{ margin: "0 0 8px" }}>Recent events</h2>
          <div className="section-rule" style={{ marginBottom: 4 }} />
          {data.events.length === 0 ? (
            <Empty label="no clicks recorded yet" />
          ) : (
            <>
              {data.events.map((e, i) => (
                <div className="event-row" key={`${e.time}-${i}`}>
                  <span className="muted">{e.time}</span>
                  <span title={e.referrer}>{e.referrer}</span>
                  <span className="ua muted" title={e.user_agent}>{e.user_agent}</span>
                  <span style={{ color: "var(--color-accent-700)" }}>{e.ip_hash}</span>
                </div>
              ))}
              <div className="mono muted" style={{ fontSize: 10.5, marginTop: 12 }}>
                ip_hash truncated to 8 chars for display
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  );
}
