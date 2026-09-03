# GoShort

A URL shortener built around one constraint: **`GET /:code` answers before anything else happens.**

The redirect reads from Redis on a warm cache, replies `302`, and only then pushes the click event
onto a buffered channel that a fixed worker pool drains in batches. If that channel is full the event
is dropped and counted rather than allowed to block the person being redirected.

**Go 1.26 · Fiber · GORM · PostgreSQL · Redis · Prometheus · React · Docker**

| | |
|---|---|
| ![The public shortener](docs/screenshots/01-shorten.png) | ![The operations dashboard](docs/screenshots/05-dashboard.png) |
| The public shortener — every figure is read live from the running process | The operations dashboard — gauges poll every 2s, aggregates every 30s |
| ![The register](docs/screenshots/06-links.png) | ![Per-link analytics](docs/screenshots/07-analytics.png) |
| The register — sorted, filtered and paginated in the browser | Per-link analytics — cache state counts down against the real Redis TTL |

---

## Architecture

```
                      ┌──────────────────────────────────────────────┐
   GET /:code  ─────► │ 1. Redis GET link:{code}      ── hit ──┐      │
                      │ 2. miss → Postgres → SET back to cache │      │
                      │                                        ▼      │
                      │ 3. 302 Found  ◄──────────── response goes out │
                      │                             FIRST, always     │
                      │ 4. click event ─► buffered channel (1000)     │
                      │       └─ full? drop + count, never block      │
                      └───────────────────┬──────────────────────────┘
                                          │
                          ┌───────────────▼────────────────┐
                          │  8 workers, batch 50 / 2s      │
                          │  CreateInBatches once          │
                          │  UPDATE click_count = +n       │  ← atomic, one per link per batch
                          └───────────────┬────────────────┘
                                          ▼
                                     PostgreSQL

   SIGINT ─► stop accepting ─► close(channel) ─► WaitGroup drain ─► close DB
```

The React SPA is built by Vite, embedded into the Go binary with `embed.FS`, and served from the
same origin as the API — one container, no nginx, and `httpOnly` cookies work without CORS.

`GET /:code` is a wildcard at the root, so route registration order matters: static assets, then
`/api/*`, `/health`, `/metrics`, then the SPA's own paths, and `/:code` last. A code that does not
resolve serves the SPA shell with a real `404` status. Custom aliases are checked against a reserved
set so nobody can mint a link at `/admin`.

---

## Benchmark

Both rows come from the **same binary**, switched only by `GOSHORT_SYNC_MODE`, which bypasses Redis
and writes the click event synchronously inside the request. Reproduce with one command:

```bash
docker compose up -d postgres redis
./load-test/run.sh
```

| Mode | req/s | p50 | p95 | p99 |
|---|---:|---:|---:|---:|
| Sync write, no cache | 3,795 | 23.12 ms | 55.17 ms | 75.11 ms |
| Async write + Redis cache | **46,146** | **1.89 ms** | **3.58 ms** | **5.90 ms** |

**p99 down 92.1%, throughput up 12.2×.**

Measured on an Apple M3 Pro (11 cores, 18 GB, macOS 26.6.2) with Postgres 16 and Redis 7 in Docker,
Go 1.26.5, k6 v2.2.0 — 100 virtual users, 30 seconds, five short codes, warm cache. Raw k6 output for
both runs is committed under [`load-test/results/`](load-test/results/) so the table can be checked
against the run that produced it.

These are single-machine numbers with the load generator on the same host as the server; they measure
the difference between the two modes, not an absolute capacity figure for a deployed instance.

---

## Running it

```bash
cp .env.example .env
docker compose up
```

Seed the administrator, then sign in at `/login`:

```bash
docker compose run --rm --entrypoint /seed api
```

The entrypoint override matters: the image's `ENTRYPOINT` is `/api`, so passing `/seed` as a command
would start the server with an ignored argument instead of running the seeder.

Demo account: `admin@goshort.dev` / `goshort-demo` (both from `.env`).

### Development

```bash
docker compose up -d postgres redis
go run ./cmd/api          # :8080
cd web && npm run dev     # :5173, proxies /api to :8080 so cookies stay same-origin
```

---

## Tests

```bash
docker compose up -d postgres redis
TEST_DATABASE_URL="postgres://goshort:goshort@localhost:5432/goshort?sslmode=disable" \
  go test -race -cover ./...
```

Integration tests skip themselves when `TEST_DATABASE_URL` is unset. There are three seams:

- **The Fiber app**, driven through `app.Test` against a real Postgres and `miniredis` — every route,
  the JWT guard, rate limiting, cache behaviour, route ordering, and a 300-goroutine concurrency test
  that proves `click_count` stays exact under `-race`.
- **The worker pool**, driven through `Enqueue` / `Start` / `Shutdown` with an injected store and an
  injected tick source, so batching, timer flushes, back-pressure drops and shutdown draining are
  tested without sleeping in assertions.
- **Playwright**, one happy path across the assembled stack.

---

## Bugs the tests found

**Request headers were being corrupted.** Values from `c.Get()` point into a buffer that fasthttp
recycles the moment the handler returns. Passing them straight into the click channel meant the
worker wrote spliced strings — a referrer of `https://news.ycombinator.com/` landed in Postgres as
`https://github.com/nator.com/`. Fixed with `utils.CopyString`. This only reproduces against a real
listener; `app.Test` builds a fresh context each call and never showed it.

**`c.Context()` was handed to GORM.** It is a `*fasthttp.RequestCtx`, valid only for the life of the
handler, and `database/sql` keeps the context in a background goroutine for as long as the rows are
open. The race detector caught the write-after-return. Every database and Redis call now takes
`c.UserContext()`.

**The connection pool was unbounded.** 300 concurrent redirects opened more connections than
Postgres' `max_connections`, and redirects started returning `500`. `SetMaxOpenConns` and friends are
now configured, so load queues on the pool instead of failing — which is also the clearest argument
for the cache: it keeps that traffic off the pool entirely.

**Expired links kept redirecting for an hour.** The expiry check lived only on the Postgres path, so
a link cached while still valid went on answering `302` until the cache entry aged out. The test that
was supposed to cover this only exercised a cold cache. The entry now carries `ExpiresAt`, and the
Redis key's TTL is capped at whatever is left of the link's life.

---

## Metrics

`/metrics` serves Prometheus text; the dashboard reads the same registry through `prometheus.Gatherer`
rather than counting anything twice.

The redirect histogram uses hand-picked buckets from 0.5 ms upward. Prometheus' default buckets start
at 5 ms, which would put every redirect in the first bucket and make p99 meaningless.

Three limits worth stating: **p99 is estimated from bucket edges**, not exact; metrics **reset on
restart**; and they are **per instance**, not cluster-wide.

`/api/stats/public` exposes hit rate, p99 and dropped counts without authentication. That is a
deliberate choice for a portfolio project — the landing page's numbers are meant to be checkable — and
not something to copy into a system where operational metrics are sensitive.

---

## API

**Public**

| Method | Path | |
|---|---|---|
| `POST` | `/api/links` | create; 20 req/min/IP |
| `GET` | `/api/links/:code/stats` | clicks, 14-day series, referrers; 60 req/min/IP |
| `GET` | `/api/stats/public` | live hit rate, p99, dropped, total clicks |
| `GET` | `/:code` | 302, or the SPA shell with a real 404 |
| `GET` | `/health`, `/metrics` | |

**Admin** — JWT (HS256, 24 h) in an `httpOnly`, `SameSite=Lax` cookie. Not `localStorage`: an XSS bug
should not be able to read the session. Every mutating route is `POST`/`DELETE`, which `Lax` already
blocks cross-site, so there is no separate CSRF token.

| Method | Path | |
|---|---|---|
| `POST` | `/api/admin/login`, `/api/admin/logout` | |
| `GET` | `/api/admin/me` | the SPA cannot read `exp` from an httpOnly cookie, so it asks |
| `GET` | `/api/admin/links` | all rows; the client sorts, filters and paginates |
| `DELETE` | `/api/admin/links/:code` | deletes and invalidates the cache immediately |
| `POST` | `/api/admin/links/:code/invalidate-cache` | |
| `GET` | `/api/admin/links/:code/analytics` | series, referrers, recent events, cache TTL |
| `GET` | `/api/admin/dashboard/summary` | live gauges, polled every 2 s |
| `GET` | `/api/admin/dashboard/overview` | heavy aggregates, cached 60 s, polled every 30 s |

Split by refresh rate on purpose: a two-second poll should not drag a `GROUP BY` along with it.

Source IPs are SHA-256 hashed before they enter the channel — no code path holds a raw address after
the handler — and only the first eight characters of a hash are ever returned to a client.
