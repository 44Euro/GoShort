import { useNavigate, useParams } from "react-router-dom";

export function NotFound() {
  const nav = useNavigate();
  const { code } = useParams();

  return (
    <div className="dark-panel notfound">
      <span className="sweep" />
      <div className="ghost" style={{ fontSize: 460 }} aria-hidden="true">
        404
      </div>

      <div style={{ position: "relative", maxWidth: 520 }}>
        <div className="eyebrow" style={{ color: "#e1ad66", marginBottom: 22 }}>
          404 · Not Found
        </div>
        <h1>This link has expired</h1>
        <p>
          {code ? (
            <>
              The code <span className="mono" style={{ color: "#e1ad66" }}>/{code}</span> was not found, or it
              passed its <span className="mono">expires_at</span> and was purged from the cache on deletion, so the
              lookup went straight to Postgres and found nothing.
            </>
          ) : (
            <>That page does not exist here.</>
          )}
        </p>
        <div style={{ display: "flex", gap: 10, justifyContent: "center", marginTop: 34, flexWrap: "wrap" }}>
          <button
            className="btn btn-primary"
            style={{ fontSize: 13.5, color: "#e1ad66", borderColor: "#e1ad66" }}
            onClick={() => nav("/")}
          >
            Shorten your own link
          </button>
          <button
            className="btn"
            style={{ fontSize: 13.5, borderColor: "rgba(243,242,242,0.28)", color: "#f3f2f2" }}
            onClick={() => nav("/s")}
          >
            Look up another code
          </button>
        </div>
      </div>

      <div className="trace">GET /{code ?? ""} → 404</div>
    </div>
  );
}
