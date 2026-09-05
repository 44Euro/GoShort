# GoShort

*[อ่านฉบับภาษาไทย →](README.th.md)*

A URL shortener built around one constraint: **`GET /:code` answers before anything else happens.**

The redirect reads from Redis on a warm cache, replies `302`, and only then pushes the click event
onto a buffered channel that a fixed worker pool drains in batches. If that channel is full the event
is dropped and counted rather than allowed to block the person being redirected.

**Go 1.26 · Fiber · GORM · PostgreSQL · Redis · Prometheus · React · Docker**

| | |
|---|---|
| ![The public shortener](docs/screenshots/01-shorten.png) | ![The operations dashboard](docs/screenshots/05-dashboard.png) |
| The public shortener — every figure is read live from the running process | Operations — live gauges only, polled every 2s |
| ![The register](docs/screenshots/06-links.png) | ![Per-link analytics](docs/screenshots/07-analytics.png) |
| The register — sorted, filtered and paginated in the browser | Per-link analytics — cache state counts down against the real Redis TTL |
| ![Aggregate analytics](docs/screenshots/10-overview.png) | ![Public link statistics](docs/screenshots/02-public-stats.png) |
| Analytics — the fourteen-day rollup, cached 60s and polled every 30s | Public statistics — no account needed to read a link's numbers |

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

## Deployment roles

The same binary runs in two roles, decided by `ADMIN_ENABLED`. On a public instance the admin
routes are **never registered**, so they answer `404` rather than `401`: the path does not exist on
that host, instead of existing and being guarded. `docker-compose.yml` runs one of each, from one
image, so the split is visible without running anything.

| | `ADMIN_ENABLED=0` | `ADMIN_ENABLED=1` |
|---|---|---|
| `POST /api/admin/login` | `404` — not in the router | `200` |
| `GET /login`, `/admin` | `404` — falls through to `/:code` | the console |
| admin JS | the console chunk is never fetched | fetched on demand, after the first paint |
| `GET /api/admin/me` on page load | not sent | sent |
| `/`, `/s/:code`, `/:code`, `/health`, `/metrics` | unchanged | unchanged |

A test asserts the 404, not the README: a `401` there would mean the route is still mounted.

```
                    same image, one variable apart
   ┌──────────────────────────────────┐   ┌──────────────────────────────────┐
   │ api        ADMIN_ENABLED=0 :8080 │   │ admin      ADMIN_ENABLED=1 :8081 │
   │                                  │   │                                  │
   │  POST /api/links                 │   │  the console, /api/admin/*       │
   │  GET  /:code            302      │   │  /login, /admin                  │
   │  /health  /health/ready          │   │  /health  /health/ready          │
   │  /metrics ◄──────────────────────┼───┼── gauges read the watched        │
   │                                  │   │   instance, not this process     │
   │  click queue   per process       │   │  click queue   idle here         │
   │  metrics       per process       │   │  metrics       nothing to see    │
   └────────────────┬─────────────────┘   └────────────────┬─────────────────┘
                    │                                      │
                    └──────────────────┬───────────────────┘
                                       ▼
                         PostgreSQL          Redis
                         rows, click_count   cache, rate limits
```

Everything below the fold is shared; everything inside a box is that process's own. The console
reads `/metrics` from the instance it watches because those figures live in that process — a
console reporting on itself would report that nothing ever happens. In a real deployment this is
Prometheus' job: it scrapes every instance and aggregates. The single hop here exists so the
console can watch one instance without standing up a metrics stack, and it does not scale past one.

The admin console is code-split out of the first bundle, and the public page skips the
`/api/admin/me` session probe entirely. **Neither is a security boundary** — the chunk is still
served as a static asset and can be fetched directly. The boundary is the server returning 404.

The SPA learns which role it is talking to from a placeholder the server substitutes into the HTML
shell once at boot. Asking the API would cost a round trip on the page that most needs to be fast.

[ADR 0001](docs/adr/0001-deployment-roles-via-one-config-flag.md) records why this is a flag rather
than two binaries, and why the console is not hosted separately — the short version is that a
static host on another domain is *cross-site*, so the `SameSite=Lax` session cookie would not be
sent at all, and getting it working means `SameSite=None` plus a hand-rolled CSRF token: more code
for a weaker posture.

---

## Running more than one instance

| | shared across instances? | |
|---|---|---|
| link cache | **yes** — Redis | one instance's warm entry is everyone's |
| rate limit counters | **yes** — Redis `INCR` per IP | the quota is global, not per replica |
| `click_count` | **yes** — `UPDATE … SET click_count = click_count + n` | atomic in the database, never read-then-write in Go |
| click queue | **no** — a Go channel per process | a drop is that process's drop, and `queue_depth` is that process's depth |
| Prometheus metrics | **no** — an in-process registry | `METRICS_SOURCE_URL` points the console at the instance it watches; aggregating more than one is Prometheus' job |
| schema migration | **serialised** — a Postgres advisory lock | see the last entry under *Bugs the tests found* |

Health checks are split along the same line, because a load balancer acts on them:

| | checks | when Redis is down | when Postgres is down |
|---|---|---|---|
| `GET /health` | nothing — liveness | `200` | `200` |
| `GET /health/ready` | Postgres, and reports Redis | `200 {"status":"degraded"}` | `503` |

**A dead cache must not empty the load balancer.** The redirect falls back to Postgres and the rate
limiter fails open, so an instance with no cache still answers correctly — just slower. Draining
every instance the moment Redis blinks turns a cache outage into a total outage. Postgres is
different: a cache miss has nothing to fall back to below it.

Liveness deliberately touches nothing. A liveness probe that pings the database restarts healthy
processes whenever the database is merely *slow*, which is the failure it exists to prevent.

**Figures belong to a process, so the console has to be told which one to watch.** Under compose
the redirects land on `api` and the console runs on `admin`; `METRICS_SOURCE_URL` points the latter
at the former, and the same code that reads this process's registry computes the numbers either way
— there is no second formula that could drift. With the console open at
`http://localhost:8081/admin`, flip the mode and watch the gauges move:

```bash
GOSHORT_SYNC_MODE=1 docker compose up -d --force-recreate api admin
for i in $(seq 1 300); do curl -s -o /dev/null localhost:8080/<code>; done   # p99 up, hit rate 0%

GOSHORT_SYNC_MODE=0 docker compose up -d --force-recreate api admin
for i in $(seq 1 300); do curl -s -o /dev/null localhost:8080/<code>; done   # p99 down, hit rate 100%
```

No figures are quoted for that because serial `curl` is not a measurement — the point is that the
difference is visible on a console running in a different process from the one being measured. The
measured numbers are in *Benchmark* below, where `load-test/run.sh` drives k6 against its own
process on `:8099` and commits its raw output.

When the watched instance cannot be reached the console answers `503`, says so in the status line,
and greys out every panel; the figures read `—`, never `0`. Zero means "nothing happened", which is
a different thing from "cannot see", and a monitoring screen that confuses the two is worse than no
screen.

The Prometheus text format is parsed with `expfmt`, which arrives with the metrics client already —
`prometheus/common` moves from an indirect dependency to a direct one, and nothing new is added to
the build.

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

That brings up **two roles from the same image**, differing only in `ADMIN_ENABLED`:

| | | |
|---|---|---|
| `api` | http://localhost:8080 | the public shortener and every `/:code` redirect. `/api/admin/*` is **not registered** on this process — it answers `404`, not `401` |
| `admin` | http://localhost:8081 | the console. Same binary, same image, one environment variable apart |

Seed the administrator, then sign in at http://localhost:8081/login:

```bash
docker compose run --rm --entrypoint /seed admin
```

The entrypoint override matters: the image's `ENTRYPOINT` is `/api`, so passing `/seed` as a command
would start the server with an ignored argument instead of running the seeder.

Demo account: `admin@goshort.dev` / `goshort-demo` (both from `.env`).

### Development

```bash
docker compose up -d postgres redis
go run ./cmd/api          # :8080, both roles in one process (ADMIN_ENABLED defaults on)
cd web && npm run dev     # :5173, proxies /api to :8080 so cookies stay same-origin
```

---

## Tests

```bash
docker compose up -d postgres redis
TEST_DATABASE_URL="postgres://goshort:goshort@localhost:5432/goshort?sslmode=disable" \
  go test -race -cover ./...
```

Integration tests skip themselves when `TEST_DATABASE_URL` is unset. There are four seams:

- **The Fiber app**, driven through `app.Test` against a real Postgres and `miniredis` — every route,
  the JWT guard, rate limiting, cache behaviour, route ordering, and a 300-goroutine concurrency test
  that proves `click_count` stays exact under `-race`.
- **The worker pool**, driven through `Enqueue` / `Start` / `Shutdown` with an injected store and an
  injected tick source, so batching, timer flushes, back-pressure drops and shutdown draining are
  tested without sleeping in assertions.
- **`model.Migrate`**, called directly against a real Postgres. It exists because migration runs
  before the Fiber app is built, so it cannot be reached through the first seam; it proves only that
  instances booting together do not collide.
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

**Two instances could not boot against a fresh database.** `AutoMigrate` asks Postgres whether an
index exists and then creates it — a plain time-of-check-to-time-of-use race. Started together,
both processes passed the check and both issued `CREATE`, and one died with
`duplicate key value violates unique constraint "pg_class_relname_nsp_index"`. It reproduced three
times out of three. Migration now runs inside a transaction holding `pg_advisory_xact_lock`, so
instances queue instead of colliding. The plain `pg_advisory_lock` is the wrong tool here: it binds
to a connection, and the unlock can be handed a different connection from the pool.

---

## Metrics

`/metrics` serves Prometheus text. Nothing is counted twice: one function turns metric families into
the figures on screen, and it is fed either by this process's `prometheus.Gatherer` or by the
metrics page of the instance being watched — see *Deployment roles*. Two sources, one formula.

The redirect histogram uses hand-picked buckets from 0.5 ms upward. Prometheus' default buckets start
at 5 ms, which would put every redirect in the first bucket and make p99 meaningless.

Two limits worth stating here: **p99 is estimated from bucket edges**, not exact, and metrics
**reset on restart**. They are also per instance — see *Running more than one instance*.

`/api/stats/public` exposes hit rate, p99 and dropped counts without authentication. That is a
deliberate choice for a portfolio project — the landing page's numbers are meant to be checkable — and
not something to copy into a system where operational metrics are sensitive.

---

## Logging

One JSON line per request, including `/:code`:

```json
{"time":"...","level":"INFO","msg":"request","method":"GET","path":"/gopher","status":302,"duration_ms":1.9,"request_id":"56c6e84519ce77e0"}
```

`X-Request-ID` is used if the caller sends one, so an id assigned upstream survives, and it comes
back on the response so a bug report can name the exact request. It is capped at 64 characters:
the value is attacker-controlled and lands in the log stream. It is copied out of the fasthttp
buffer before being stored, for the same reason the referrer was — see *Bugs the tests found*.

GORM's own logger is routed through `slog` too, with colour off and `ErrRecordNotFound` ignored.
Before that, every 404 redirect emitted a coloured multi-line block on stderr, which made "the
logs are JSON" false in practice. `LOG_FORMAT=text` (the default) is for reading in a terminal;
`LOG_FORMAT=json` is for anything that indexes fields.

Worker-pool lines carry no request id. Click events are batched across requests, so the
one-to-one relationship a request id implies does not exist there.

The access log runs on the redirect path with no sampling. Its cost was **not measured** — no claim
is made about it, and it is the first thing to sample if it ever shows up in a profile.

---

## API

**Public**

| Method | Path | |
|---|---|---|
| `POST` | `/api/links` | create; 20 req/min/IP |
| `GET` | `/api/links/:code/stats` | clicks, 14-day series, referrers; 60 req/min/IP |
| `GET` | `/api/stats/public` | live hit rate, p99, dropped, total clicks |
| `GET` | `/:code` | 302, or the SPA shell with a real 404 |
| `GET` | `/health` | liveness — `200` while the process is alive, checks nothing |
| `GET` | `/health/ready` | readiness — `503` without Postgres, `200 degraded` without Redis |
| `GET` | `/metrics` | |

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

---

## What this is not

This is a portfolio project, not a service. It is not hosted anywhere on purpose: a public URL
shortener with no abuse controls is a liability, not a demo.

These are the gaps, with what closing each would actually take:

**Schema changes have no story.** `AutoMigrate` runs at boot. Concurrent boots are now safe, but
there are no versioned migrations, no rollback, and nothing that would let a schema change land
under live traffic. Closing it means a migration tool, a migrations table, and an expand/contract
discipline for every column change.

**Nothing pages anyone.** Dropped click events are counted and logged, and `/metrics` exposes them,
but there is no alerting and no SLO, so "we lost events for an hour" is only visible to someone who
happens to look. Closing it means Prometheus and Alertmanager, an error budget worth defending, and
a person on call.

**Tracing stops at this process.** A request id ties one process's log lines together and no
further. Closing it means OpenTelemetry, a collector, and somewhere to keep the traces.

**Links are anonymous and unowned.** Anyone can create one, a single seeded administrator sees all
of them, and nothing checks where a link points. A real shortener needs phishing and malware
scanning at creation time, an abuse reporting path, and per-account ownership before it can be
opened to the public. That is the single largest reason this is not.
