import type { DayPoint, Referrer } from "../api";

const ink = "var(--color-text)";
const accent = "var(--color-accent)";

function niceMax(values: number[]): number {
  const peak = Math.max(1, ...values);
  const step = Math.pow(10, Math.floor(Math.log10(peak)));
  return Math.ceil(peak / step) * step;
}

export function Empty({ label }: { label: string }) {
  return (
    <div
      className="mono muted"
      style={{ padding: "28px 0", fontSize: 11, letterSpacing: "0.08em", textTransform: "uppercase" }}
    >
      {label}
    </div>
  );
}

/** กราฟแท่งรายวัน — scale คำนวณจากข้อมูลจริง ไม่ fix เพดานไว้ */
export function DailyBars({ points, height = 190 }: { points: DayPoint[]; height?: number }) {
  if (points.length === 0) return <Empty label="no clicks recorded yet" />;

  const max = niceMax(points.map((p) => p.clicks));
  const w = 1000;
  const gap = 8;
  const bw = (w - gap * (points.length - 1)) / points.length;

  return (
    <div>
      <svg
        viewBox={`0 0 ${w} ${height}`}
        preserveAspectRatio="none"
        style={{ width: "100%", height, display: "block" }}
        role="img"
        aria-label={`Daily clicks over the last ${points.length} days`}
      >
        <line x1="0" y1={height - 0.5} x2={w} y2={height - 0.5} stroke={ink} strokeWidth="1" opacity="0.3" />
        {points.map((p, i) => {
          const h = (p.clicks / max) * (height - 12);
          return (
            <rect
              key={p.day}
              x={i * (bw + gap)}
              y={height - h}
              width={bw}
              height={Math.max(h, p.clicks > 0 ? 2 : 0)}
              fill={i === points.length - 1 ? accent : ink}
              opacity={i === points.length - 1 ? 1 : 0.72}
            >
              <title>{`${p.day}: ${p.clicks.toLocaleString()} clicks`}</title>
            </rect>
          );
        })}
      </svg>
      <div style={{ display: "flex", justifyContent: "space-between", marginTop: 10 }}>
        {points.map((p) => (
          <span key={p.day} className="mono muted" style={{ fontSize: 9, letterSpacing: "0.04em" }}>
            {p.day.slice(8)}
          </span>
        ))}
      </div>
    </div>
  );
}

/** เส้นย่อในแถวตาราง ไม่มีแกน ไม่มี label */
export function Sparkline({ values, width = 96, height = 22 }: { values: number[]; width?: number; height?: number }) {
  if (values.length === 0) return null;
  const max = Math.max(1, ...values);
  const step = width / Math.max(1, values.length - 1);
  const d = values
    .map((v, i) => `${i === 0 ? "M" : "L"}${(i * step).toFixed(1)},${(height - (v / max) * (height - 2)).toFixed(1)}`)
    .join(" ");

  return (
    <svg width={width} height={height} style={{ display: "block", overflow: "visible" }} aria-hidden="true">
      <path d={d} fill="none" stroke={ink} strokeWidth="1.2" opacity="0.75" />
    </svg>
  );
}

export function ReferrerBars({ items }: { items: Referrer[] }) {
  if (items.length === 0) return <Empty label="no referrers yet" />;
  const top = Math.max(...items.map((r) => r.percent), 1);

  return (
    <div>
      {items.map((r) => (
        <div
          key={r.name}
          style={{
            display: "grid",
            gridTemplateColumns: "minmax(0,1fr) 90px 52px",
            gap: 14,
            alignItems: "center",
            padding: "9px 0",
            borderBottom: "1px solid var(--color-divider)",
          }}
        >
          <span
            className="mono"
            style={{ fontSize: 12, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}
            title={r.name}
          >
            {r.name}
          </span>
          <span style={{ height: 4, background: "var(--color-neutral-300)", display: "block" }}>
            <span
              style={{
                display: "block",
                height: "100%",
                width: `${(r.percent / top) * 100}%`,
                background: accent,
              }}
            />
          </span>
          <span className="mono muted" style={{ fontSize: 11, textAlign: "right" }}>
            {r.percent.toFixed(1)}%
          </span>
        </div>
      ))}
    </div>
  );
}
