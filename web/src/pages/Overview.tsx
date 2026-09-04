import { Link } from "react-router-dom";

import { api, type Overview as Data } from "../api";
import { DailyBars, Empty, ReferrerBars } from "../components/Charts";
import { usePoll } from "../usePoll";

export function Overview() {
  // ชุดนี้เปิด GROUP BY ทุกครั้ง แคชไว้ 60 วิที่ Redis อยู่แล้ว poll ถี่กว่านี้ไม่ได้อะไรเพิ่ม
  const { data, loading, error } = usePoll<Data>(() => api.get<Data>("/api/admin/dashboard/overview"), 30000);

  const series = data?.series ?? [];
  const total = series.reduce((sum, p) => sum + p.clicks, 0);
  const busiest = series.reduce<number>((peak, p) => Math.max(peak, p.clicks), 0);

  return (
    <div className="rise">
      <div className="page-head">
        <div>
          <div className="label-mono" style={{ letterSpacing: "0.22em", marginBottom: 10 }}>
            GET /api/admin/dashboard/overview
          </div>
          <h1>Fourteen days of traffic</h1>
        </div>
        <span className="mono muted" style={{ fontSize: 11, letterSpacing: "0.1em", textTransform: "uppercase" }}>
          cached 60s · refreshed every 30s
        </span>
      </div>

      <div className="tile-row" style={{ marginTop: 30 }}>
        <div className="tile">
          <div className="tile-label">Clicks in window</div>
          <div className="tile-value">{loading ? "—" : total.toLocaleString()}</div>
        </div>
        <div className="tile">
          <div className="tile-label">Busiest day</div>
          <div className="tile-value">{loading ? "—" : busiest.toLocaleString()}</div>
        </div>
        <div className="tile">
          <div className="tile-label">Mean per day</div>
          <div className="tile-value">
            {loading || series.length === 0 ? "—" : (total / series.length).toFixed(1)}
          </div>
        </div>
        <div className="tile">
          <div className="tile-label">Links ranked</div>
          <div className="tile-value">{loading ? "—" : (data?.top_links.length ?? 0)}</div>
        </div>
      </div>

      <div className="panel">
        <div className="panel-head">
          <h2>Clicks, last fourteen days</h2>
          <span className="mono muted" style={{ fontSize: 11 }}>batched insert · 2s flush</span>
        </div>
        {error && <div className="form-error">{error}</div>}
        {loading ? <Empty label="loading" /> : <DailyBars points={series} />}
      </div>

      <div className="two-col">
        <div>
          <h2 style={{ margin: "0 0 8px" }}>Top five links</h2>
          <div className="section-rule" style={{ marginBottom: 4 }} />
          {(data?.top_links ?? []).length === 0 ? (
            <Empty label={loading ? "loading" : "no links yet"} />
          ) : (
            data!.top_links.map((l) => (
              <Link key={l.code} to={`/admin/links/${encodeURIComponent(l.code)}`} className="rank-row">
                <span className="mono muted num" style={{ fontSize: 11 }}>{l.rank}</span>
                <span className="mono" style={{ fontSize: 13, color: "var(--color-accent-700)" }}>/{l.code}</span>
                <span className="num" style={{ fontSize: 17 }}>{l.clicks.toLocaleString()}</span>
              </Link>
            ))
          )}
        </div>

        <div>
          <h2 style={{ margin: "0 0 8px" }}>Top referrers</h2>
          <div className="section-rule" style={{ marginBottom: 4 }} />
          <ReferrerBars items={data?.referrers ?? []} />
          <p className="dim" style={{ fontSize: 13, lineHeight: 1.7, marginTop: 18 }}>
            Referrers are read from the batched <span className="mono">click_events</span> table. Source IPs are
            SHA-256 hashed before insert and are never stored in the clear.
          </p>
        </div>
      </div>
    </div>
  );
}
