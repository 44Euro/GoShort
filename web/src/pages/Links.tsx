import { useMemo, useState } from "react";
import { Link } from "react-router-dom";

import { api, type AdminLink } from "../api";
import { Sparkline } from "../components/Charts";
import { ConfirmDialog } from "../components/ConfirmDialog";
import { usePoll } from "../usePoll";

type SortKey = "code" | "clicks" | "created_at";
const PAGE_SIZE = 8;

export function Links() {
  // ยอดคลิกอ่านสดทุกรอบ ส่วนเส้นกราฟ 14 วันฝั่งเซิร์ฟเวอร์ cache ไว้ การ poll ถี่
  // จึงไม่ลาก GROUP BY ไปด้วย — คลิกจะโผล่ช้าสุดราวจังหวะนี้บวกอีก 2 วิที่ worker flush
  const { data, error, loading, reload } = usePoll<{ links: AdminLink[] }>(
    () => api.get<{ links: AdminLink[] }>("/api/admin/links"),
    5000,
  );

  const [query, setQuery] = useState("");
  const [sort, setSort] = useState<SortKey>("clicks");
  const [desc, setDesc] = useState(true);
  const [page, setPage] = useState(1);
  const [busy, setBusy] = useState<string | null>(null);
  const [failure, setFailure] = useState<string | null>(null);
  const [pending, setPending] = useState<AdminLink | null>(null);

  const all = data?.links ?? [];

  // sort/filter/paginate ทำฝั่ง client ทั้งหมด ข้อมูล demo น้อยและ API คืนมาทีเดียวอยู่แล้ว
  const rows = useMemo(() => {
    const q = query.trim().toLowerCase();
    const filtered = q
      ? all.filter((l) => l.code.toLowerCase().includes(q) || l.long_url.toLowerCase().includes(q))
      : all;

    const sorted = [...filtered].sort((a, b) => {
      const cmp =
        sort === "clicks"
          ? a.clicks - b.clicks
          : sort === "code"
            ? a.code.localeCompare(b.code)
            : a.created_at.localeCompare(b.created_at);
      return desc ? -cmp : cmp;
    });
    return sorted;
  }, [all, query, sort, desc]);

  const pages = Math.max(1, Math.ceil(rows.length / PAGE_SIZE));
  const current = Math.min(page, pages);
  const shown = rows.slice((current - 1) * PAGE_SIZE, current * PAGE_SIZE);

  const changeSort = (key: SortKey) => {
    setPage(1);
    if (key === sort) setDesc((d) => !d);
    else {
      setSort(key);
      setDesc(true);
    }
  };

  const arrow = (key: SortKey) => (sort === key ? (desc ? "↓" : "↑") : "");

  const remove = async () => {
    if (!pending) return;
    const { code } = pending;
    setBusy(code);
    setFailure(null);
    try {
      await api.del(`/api/admin/links/${encodeURIComponent(code)}`);
      setPending(null);
      reload();
    } catch (err) {
      setPending(null);
      setFailure(`Could not delete /${code}: ${(err as Error).message}`);
    } finally {
      setBusy(null);
    }
  };

  return (
    <div className="rise">
      <div className="page-head">
        <div>
          <div className="label-mono" style={{ letterSpacing: "0.22em", marginBottom: 10 }}>
            GET /api/admin/links
          </div>
          <h1>The register</h1>
        </div>
        <input
          className="input mono"
          style={{ width: 240, fontSize: 12.5 }}
          placeholder="Filter by code or host…"
          value={query}
          onChange={(e) => {
            setQuery(e.target.value);
            setPage(1);
          }}
          aria-label="Filter links"
        />
      </div>

      <div className="page-body">
        <div className="table-meta">
          <span>{rows.length} LINKS</span>
          <span className="sep">│</span>
          <span>SORTED BY {sort === "created_at" ? "CREATED" : sort.toUpperCase()}</span>
          <span className="sep">│</span>
          <span>PAGE {current} OF {pages}</span>
        </div>

        {failure && <div className="form-error">{failure}</div>}

        {loading && <div className="mono muted" style={{ padding: "40px 0" }}>Loading…</div>}
        {error && !loading && <div className="form-error">{error}</div>}

        {!loading && rows.length === 0 && (
          <div className="mono muted" style={{ padding: "40px 0" }}>
            {all.length === 0 ? "No links yet — create one from the public page." : "Nothing matches that filter."}
          </div>
        )}

        {shown.length > 0 && (
          <div className="table-scroll">
            <table className="table" style={{ fontSize: 13.5 }}>
              <thead>
                <tr>
                  <th className="sortable" style={{ width: 132 }} onClick={() => changeSort("code")}>
                    Code {arrow("code")}
                  </th>
                  <th>Destination</th>
                  <th className="sortable" style={{ textAlign: "right", width: 108 }} onClick={() => changeSort("clicks")}>
                    Clicks {arrow("clicks")}
                  </th>
                  <th style={{ width: 130 }}>Last 14d</th>
                  <th className="sortable" style={{ width: 118 }} onClick={() => changeSort("created_at")}>
                    Created {arrow("created_at")}
                  </th>
                  <th style={{ width: 96 }}>Status</th>
                  <th style={{ width: 72 }} />
                </tr>
              </thead>
              <tbody>
                {shown.map((l) => (
                  <tr key={l.code}>
                    <td>
                      <Link
                        to={`/admin/links/${encodeURIComponent(l.code)}`}
                        className="mono"
                        style={{ fontSize: 13, fontWeight: 500, textDecoration: "none" }}
                      >
                        /{l.code}
                      </Link>
                    </td>
                    <td style={{ maxWidth: 0 }}>
                      <span className="cell-url" title={l.long_url}>{l.long_url}</span>
                    </td>
                    <td className="cell-clicks">{l.clicks.toLocaleString()}</td>
                    <td><Sparkline values={l.last_14_days} /></td>
                    <td className="mono muted" style={{ fontSize: 11.5 }}>{l.created_at.slice(0, 10)}</td>
                    <td>
                      <span className={l.status === "active" ? "tag tag-active" : "tag tag-expired"}>
                        {l.status}
                      </span>
                    </td>
                    <td style={{ textAlign: "right" }}>
                      <button
                        className="btn btn-ghost"
                        style={{ fontSize: 12 }}
                        onClick={() => setPending(l)}
                        disabled={busy === l.code}
                      >
                        {busy === l.code ? "…" : "Delete"}
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {rows.length > 0 && (
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginTop: 22, gap: 12 }}>
            <span className="mono muted" style={{ fontSize: 11 }}>
              Rows {(current - 1) * PAGE_SIZE + 1}–{Math.min(current * PAGE_SIZE, rows.length)} of {rows.length}
            </span>
            <div style={{ display: "flex", gap: 8 }}>
              <button
                className="btn btn-secondary"
                style={{ fontSize: 12.5 }}
                disabled={current <= 1}
                onClick={() => setPage(current - 1)}
              >
                Previous
              </button>
              <button
                className="btn btn-secondary"
                style={{ fontSize: 12.5 }}
                disabled={current >= pages}
                onClick={() => setPage(current + 1)}
              >
                Next
              </button>
            </div>
          </div>
        )}
      </div>

      <ConfirmDialog
        open={pending !== null}
        kicker={pending ? `DELETE /api/admin/links/${pending.code}` : undefined}
        title={pending ? `Delete /${pending.code}?` : ""}
        body={
          pending && (
            <>
              The row is removed and its Redis key is dropped in the same request, so{" "}
              <span className="mono">/{pending.code}</span> stops resolving immediately rather than at the end
              of its TTL. Its {pending.clicks.toLocaleString()} recorded{" "}
              {pending.clicks === 1 ? "click goes" : "clicks go"} with it. This cannot be undone.
            </>
          )
        }
        confirmLabel="Delete the link"
        busy={busy !== null}
        onConfirm={remove}
        onCancel={() => setPending(null)}
      />
    </div>
  );
}
