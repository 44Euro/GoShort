import { useEffect, useRef, useState } from "react";

import { api, type Summary } from "../api";
import { usePoll } from "../usePoll";

const QUEUE_TICKS = 28;

export function Dashboard() {
  // เกจสดอย่างเดียว ภาพรวมที่เปิด GROUP BY ย้ายไปหน้า /admin/analytics แล้ว
  // ไม่งั้นการ poll ทุก 2 วิจะลากคิวรีหนักไปด้วยทุกครั้ง
  const live = usePoll<Summary>(() => api.get<Summary>("/api/admin/dashboard/summary"), 2000);

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
  // ศูนย์แปลว่า "ไม่มีอะไรเกิดขึ้น" ซึ่งคนละเรื่องกับ "ยังอ่านค่าไม่ได้"
  const count = (v: number | undefined) => (v === undefined ? "—" : v.toLocaleString());
  const blind = live.error !== null;

  return (
    <div className="rise">
      <div style={{ padding: "40px 40px 26px", display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 30, flexWrap: "wrap" }}>
        <div style={{ maxWidth: 640 }}>
          <div className="label-mono" style={{ letterSpacing: "0.22em", marginBottom: 10 }}>
            Live instrumentation
          </div>
          <h1 style={{ margin: "0 0 14px" }}>The redirect path, under observation</h1>
          <p className="dim" style={{ margin: 0, fontSize: 14.5, lineHeight: 1.75 }}>
            Every figure below is read from the running process: cache-aside hits against Redis, the depth of
            the buffered channel feeding the click-writer pool, and the tail latency of{" "}
            <span className="mono">GET /:code</span>.
          </p>
        </div>
        <div className="mono muted" style={{ display: "flex", alignItems: "center", gap: 10, marginTop: 6, fontSize: 11, letterSpacing: "0.1em", textTransform: "uppercase" }}>
          <span className="pulse-dot" />
          <span>{live.error ? live.error : live.loading ? "connecting" : "live · 2s"}</span>
        </div>
      </div>

      <div className={blind ? "instrument is-stale" : "instrument"}>
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
              <span style={{ color: s && s.dropped_events > 0 ? "var(--color-ink-accent)" : undefined }}>
                {count(s?.dropped_events)} dropped
              </span>
            </div>
          </div>

          <div className="gauge">
            <div className="gauge-label">Total clicks</div>
            <div className="gauge-value"><b>{count(s?.total_clicks)}</b></div>
            <div style={{ height: 3, marginTop: 18 }} />
            <div className="gauge-foot">
              <span style={{ color: "var(--color-ink-accent)" }}>+{sinceLoad}</span> since this page loaded
            </div>
          </div>
        </div>
      </div>

      <div className={blind ? "panel is-stale" : "panel"}>
        <div className="panel-head">
          <h2>The write path</h2>
          <span className="mono muted" style={{ fontSize: 11 }}>GET /api/admin/dashboard/summary</span>
        </div>
        <div className="write-path">
          <div>
            <p className="dim" style={{ fontSize: 14, lineHeight: 1.75 }}>
              The redirect answers with a 302 before anything is written. The click event is pushed onto a
              buffered channel with a <span className="mono">select</span> and a{" "}
              <span className="mono">default</span> arm, so a full channel drops the event and logs it rather
              than making the person being redirected wait on Postgres.
            </p>
            <p className="dim" style={{ fontSize: 14, lineHeight: 1.75, margin: 0 }}>
              Eight workers drain that channel, accumulate up to fifty events, and write them in a single{" "}
              <span className="mono">CreateInBatches</span>. Click counts move by{" "}
              <span className="mono">click_count = click_count + ?</span> so concurrent workers touching one
              link cannot lose an increment.
            </p>
          </div>
          <div>
            <div className="figure-row">
              <span>Events written</span>
              <b className="num">{count(s?.written_events)}</b>
            </div>
            <div className="figure-row">
              <span>Events dropped</span>
              <b className="num">{count(s?.dropped_events)}</b>
            </div>
            <div className="figure-row">
              <span>Queue depth</span>
              <b className="num">
                {s ? `${s.queue_depth} / ${s.queue_capacity}` : "—"}
              </b>
            </div>
            <div className="figure-row">
              <span>Clicks recorded</span>
              <b className="num">{count(s?.total_clicks)}</b>
            </div>
          </div>
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
      <polyline points={points} fill="none" stroke="var(--color-ink-accent)" strokeWidth="1.25" strokeLinejoin="round" vectorEffect="non-scaling-stroke" />
    </svg>
  );
}
