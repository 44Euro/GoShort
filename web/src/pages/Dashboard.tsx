import { useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";

import { api, type Overview, type Summary } from "../api";
import { DailyBars, Empty, ReferrerBars } from "../components/Charts";
import { usePoll } from "../usePoll";

const QUEUE_TICKS = 28;

export function Dashboard() {
  // เกจสดกับภาพรวมหนักรีเฟรชคนละจังหวะ ไม่งั้นการ poll ทุก 2 วิจะลาก GROUP BY ไปด้วย
  const live = usePoll<Summary>(() => api.get<Summary>("/api/admin/dashboard/summary"), 2000);
  const over = usePoll<Overview>(() => api.get<Overview>("/api/admin/dashboard/overview"), 30000);

  const s = live.data;
  const baseline = useRef<number | null>(null);
  const [latency, setLatency] = useState<number[]>([]);

  useEffect(() => {
    if (!s) return;
    if (baseline.current === null) baseline.current = s.total_clicks;
    setLatency((prev) => [...prev, s.p99_redirect_ms].slice(-40));
  }, [s]);

  const sinceLoad = s && baseline.current !== null ? s.total_clicks - baseline.current : 0;
  const queueFilled = s ? Math.round((s.queue_depth / Math.max(1, s.queue_capacity)) * QUEUE_TICKS) : 0;
  const num = (v: number | undefined, digits = 1) => (v === undefined ? "—" : v.toFixed(digits));

  return (
    <div className="rise">
      <div style={{ padding: "40px 40px 26px", display: "flex", justifyContent: "space-between", gap: 30, flexWrap: "wrap" }}>
        <div style={{ maxWidth: 640 }}>
          <div className="label-mono" style={{ letterSpacing: "0.22em", marginBottom: 10 }}>
            Live instrumentation
          </div>
          <h1 style={{ fontSize: 46, fontWeight: 400, margin: "0 0 14px", letterSpacing: "-0.025em" }}>
            The redirect path, under observation
          </h1>
          <p className="dim" style={{ margin: 0, fontSize: 14.5, lineHeight: 1.7 }}>
            Every figure below is read from the running process: cache-aside hits against Redis, the depth of
            the buffered channel feeding the click-writer pool, and the tail latency of{" "}
            <span className="mono">GET /:code</span>.
          </p>
        </div>
        <div className="mono muted" style={{ display: "flex", alignItems: "center", gap: 10, fontSize: 11, letterSpacing: "0.1em", textTransform: "uppercase" }}>
          <span className="pulse-dot" />
          <span>{live.error ? "API unreachable" : live.loading ? "connecting" : "live · 2s"}</span>
        </div>
      </div>

      <div className="instrument">
        <span className="sweep" />
        <div className="instrument-grid">
          <div className="gauge">
            <div className="gauge-label">Cache hit rate</div>
            <div className="gauge-value"><b>{num(s?.cache_hit_rate)}</b><i>%</i></div>
            <div className="meter"><span style={{ width: `${s?.cache_hit_rate ?? 0}%` }} /></div>
            <div className="gauge-foot">link:{"{code}"} &nbsp;·&nbsp; TTL 1h</div>
          </div>

          <div className="gauge">
            <div className="gauge-label">p99 redirect</div>
            <div className="gauge-value"><b>{num(s?.p99_redirect_ms, 2)}</b><i>ms</i></div>
            <LatencyTrace values={latency} />
            <div className="gauge-foot">estimated from histogram buckets</div>
          </div>

          <div className="gauge">
            <div className="gauge-label">Worker queue</div>
            <div className="gauge-value">
              <b>{s?.queue_depth ?? "—"}</b>
              <i>/ {s?.queue_capacity ?? "—"}</i>
            </div>
            <div className="queue-ticks">
              {Array.from({ length: QUEUE_TICKS }, (_, i) => (
                <span key={i} className={i < queueFilled ? "on" : undefined} />
              ))}
            </div>
            <div className="gauge-foot">
              8 workers &nbsp;·&nbsp; batch 50 &nbsp;·&nbsp;{" "}
              <span style={{ color: s && s.dropped_events > 0 ? "#e1ad66" : undefined }}>
                {s?.dropped_events ?? 0} dropped
              </span>
            </div>
          </div>

          <div className="gauge">
            <div className="gauge-label">Total clicks</div>
            <div className="gauge-value"><b>{(s?.total_clicks ?? 0).toLocaleString()}</b></div>
            <div style={{ height: 3, marginTop: 18 }} />
            <div className="gauge-foot">
              <span style={{ color: "#e1ad66" }}>+{sinceLoad}</span> since this page loaded
            </div>
          </div>
        </div>
      </div>

      <div className="panel">
        <div className="panel-head">
          <h2>Clicks, last fourteen days</h2>
          <span className="mono muted" style={{ fontSize: 11 }}>batched insert · 2s flush</span>
        </div>
        {over.loading ? <Empty label="loading" /> : <DailyBars points={over.data?.series ?? []} />}
      </div>

      <div className="two-col">
        <div>
          <h2 style={{ fontSize: 25, margin: "0 0 8px" }}>Top five links</h2>
          <div className="section-rule" style={{ borderBottom: "1px solid var(--color-text)", paddingBottom: 8, marginBottom: 4 }} />
          {(over.data?.top_links ?? []).length === 0 ? (
            <Empty label="no links yet" />
          ) : (
            over.data!.top_links.map((l) => (
              <Link
                key={l.code}
                to={`/admin/links/${encodeURIComponent(l.code)}`}
                style={{
                  display: "grid",
                  gridTemplateColumns: "22px 1fr auto",
                  gap: 12,
                  alignItems: "baseline",
                  padding: "11px 0",
                  borderBottom: "1px solid var(--color-divider)",
                  textDecoration: "none",
                  color: "inherit",
                }}
              >
                <span className="mono muted" style={{ fontSize: 11 }}>{l.rank}</span>
                <span className="mono" style={{ fontSize: 13, color: "var(--color-accent-700)" }}>/{l.code}</span>
                <span style={{ fontFamily: "var(--font-heading)", fontSize: 17, fontFeatureSettings: "'tnum'" }}>
                  {l.clicks.toLocaleString()}
                </span>
              </Link>
            ))
          )}
        </div>

        <div>
          <h2 style={{ fontSize: 25, margin: "0 0 8px" }}>Top referrers</h2>
          <div className="section-rule" style={{ borderBottom: "1px solid var(--color-text)", paddingBottom: 8, marginBottom: 4 }} />
          <ReferrerBars items={over.data?.referrers ?? []} />
          <p className="dim" style={{ fontSize: 13, lineHeight: 1.65, marginTop: 18 }}>
            Referrers are read from the batched <span className="mono">click_events</span> table. Source IPs are
            SHA-256 hashed before insert and are never stored in the clear.
          </p>
        </div>
      </div>
    </div>
  );
}

function LatencyTrace({ values }: { values: number[] }) {
  if (values.length < 2) return <div style={{ height: 34, marginTop: 12 }} />;
  const max = Math.max(...values, 1);
  const step = 240 / (values.length - 1);
  const points = values.map((v, i) => `${(i * step).toFixed(1)},${(34 - (v / max) * 32).toFixed(1)}`).join(" ");

  return (
    <svg
      viewBox="0 0 240 34"
      preserveAspectRatio="none"
      style={{ width: "100%", height: 34, marginTop: 12, display: "block", overflow: "visible" }}
      aria-hidden="true"
    >
      <polyline points={points} fill="none" stroke="#e1ad66" strokeWidth="1.25" strokeLinejoin="round" vectorEffect="non-scaling-stroke" />
    </svg>
  );
}
