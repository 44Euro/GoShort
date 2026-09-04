import { useState } from "react";

import { ApiError, api, type CreatedLink, type PublicStats } from "../api";
import { PublicNav } from "../components/Nav";
import { usePoll } from "../usePoll";

type Errors = { url?: string; alias?: string; form?: string };

function statValue(v: number | undefined, digits: number, unit: string) {
  if (v === undefined) return <>—<small> {unit}</small></>;
  return (
    <>
      {v.toFixed(digits)}
      <small> {unit}</small>
    </>
  );
}

export function Shorten() {
  const [url, setUrl] = useState("");
  const [alias, setAlias] = useState("");
  const [result, setResult] = useState<CreatedLink | null>(null);
  const [errors, setErrors] = useState<Errors>({});
  const [busy, setBusy] = useState(false);
  const [copied, setCopied] = useState(false);

  // ตัวเลขบนหน้านี้อ่านจาก process ที่รันอยู่จริง ไม่มีค่าไหน hardcode
  const stats = usePoll<PublicStats>(() => api.get<PublicStats>("/api/stats/public"), 5000);
  const s = stats.data;

  // ปล่อยให้ error ค้างอยู่ระหว่างที่ผู้ใช้กำลังแก้ตรงจุดที่ผิด อ่านแล้วสับสน
  const clearError = (field: keyof Errors) =>
    setErrors((prev) => (prev[field] || prev.form ? { ...prev, [field]: undefined, form: undefined } : prev));

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setErrors({});
    setResult(null);
    try {
      const created = await api.post<CreatedLink>("/api/links", {
        long_url: url,
        alias: alias || undefined,
      });
      setResult(created);
      setCopied(false);
    } catch (err) {
      setErrors(explain(err));
    } finally {
      setBusy(false);
    }
  };

  const copy = async () => {
    if (!result) return;
    await navigator.clipboard.writeText(result.short_url);
    setCopied(true);
    setTimeout(() => setCopied(false), 1600);
  };

  return (
    <div className="rise">
      <PublicNav />

      <div className="shell">
        <h1 className="display">Short links that never make anyone wait.</h1>

        <div className="lede">
          <p>
            Paste a long URL and get a code back. The redirect that follows never touches Postgres on a
            warm cache, and never waits for analytics to be written — the click event is pushed onto a
            buffered channel and picked up by a pool of workers that batch their inserts.
          </p>
          <p>
            If that channel ever fills, the event is dropped and logged rather than allowed to block the
            person being redirected. That rule is the whole design: the response goes out first, always.
            Creation is rate limited to twenty requests per minute per address.
          </p>
        </div>

        <form className="rule-band" onSubmit={submit} noValidate>
          <div className="create-row">
            <div className="field">
              <label className="label-mono" htmlFor="long-url">Long URL</label>
              <input
                id="long-url"
                className="input input-rule"
                placeholder="https://example.com/a/very/long/path"
                value={url}
                onChange={(e) => {
                  setUrl(e.target.value);
                  clearError("url");
                }}
                aria-invalid={!!errors.url}
              />
              {errors.url && <div className="field-error">{errors.url}</div>}
            </div>

            <div className="field">
              <label className="label-mono" htmlFor="alias">
                Custom alias&nbsp;&nbsp;
                <span style={{ textTransform: "none", letterSpacing: 0, color: "var(--color-neutral-500)" }}>
                  optional
                </span>
              </label>
              <input
                id="alias"
                className="input input-rule"
                placeholder="my-code"
                value={alias}
                onChange={(e) => {
                  setAlias(e.target.value);
                  clearError("alias");
                }}
                aria-invalid={!!errors.alias}
              />
              {errors.alias && <div className="field-error">{errors.alias}</div>}
            </div>

            <button
              className="btn btn-primary"
              style={{ minHeight: 42, fontSize: 14, paddingInline: 26 }}
              disabled={busy}
            >
              {busy ? "Shortening…" : "Shorten"}
            </button>
          </div>
          {errors.form && <div className="form-error">{errors.form}</div>}
        </form>

        {result && (
          <div className="result-card">
            <div>
              <div className="label-mono" style={{ marginBottom: 8, letterSpacing: "0.18em" }}>
                201 Created
              </div>
              <div className="result-url">{result.short_url.replace(/^https?:\/\//, "")}</div>
            </div>
            <div style={{ display: "flex", alignItems: "center", gap: 16 }}>
              <span className="mono muted" style={{ fontSize: 11 }}>
                {s ? `cached · 302 in ${s.p99_redirect_ms.toFixed(1)} ms` : "cached · 302"}
              </span>
              <button
                className="btn btn-secondary"
                onClick={copy}
                style={{ fontSize: 13, minWidth: 96 }}
                type="button"
              >
                {copied ? "Copied" : "Copy"}
              </button>
            </div>
          </div>
        )}

        <div className="stat-grid">
          <div className="stat">
            <div className="stat-value">{statValue(s?.p99_redirect_ms, 1, "ms")}</div>
            <span className="label-mono">p99 redirect</span>
            <p>
              {stats.error
                ? "The API is not reachable, so this figure is unavailable."
                : "Read live from the running process — a histogram over every redirect this instance has served."}
            </p>
          </div>
          <div className="stat">
            <div className="stat-value">{statValue(s?.cache_hit_rate, 1, "%")}</div>
            <span className="label-mono">Cache hit rate</span>
            <p>
              Cache-aside on a one hour TTL, invalidated the moment a link is deleted rather than left to
              expire.
            </p>
          </div>
          <div className="stat">
            <div className="stat-value">
              {s === null ? <>—<small> dropped</small></> : <>{s.dropped_events.toLocaleString()}<small> dropped</small></>}
            </div>
            <span className="label-mono">Click events</span>
            <p>Eight workers drain a thousand-slot channel in batches of fifty, flushing every two seconds.</p>
          </div>
        </div>
      </div>
    </div>
  );
}

function explain(err: unknown): Errors {
  if (!(err instanceof ApiError)) return { form: "Something went wrong. Try again." };

  if (err.status === 429) {
    return { form: "You have created links too quickly. The limit is twenty per minute — wait a moment and try again." };
  }
  if (err.status === 409) return { alias: err.message };
  if (err.status === 400) {
    return err.message.toLowerCase().includes("alias") ? { alias: err.message } : { url: err.message };
  }
  return { form: err.message };
}
