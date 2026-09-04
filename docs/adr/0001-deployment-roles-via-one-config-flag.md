# Deployment roles come from one config flag, not from separate services

`ADMIN_ENABLED` decides whether a process registers the admin console at all. On a public
instance the admin routes are never added to the router, so `/api/admin/login` answers `404` —
the path does not exist there, rather than existing and being guarded. Both roles ship in the
same binary and the same image; `docker-compose.yml` runs one of each.

## Considered options

**Two binaries sharing `internal/`.** Produces exactly the behaviour the flag produces, but costs
changes to the Dockerfile, compose, CI, the e2e base URL, and the architecture diagram — and
invites the fair question of whether two services against one database are a distributed monolith.
Nothing about the running system would differ.

**The admin console on a separate static host** (Vercel or similar), calling the API cross-origin.
Rejected on cookie mechanics, which are worth stating precisely because the failure is not obvious:

- `admin.example.com` and `api.example.com` are *same-site*. A `SameSite=Lax` cookie is sent
  between them. This case is fine.
- `goshort-admin.vercel.app` and `api.example.com` are different registrable domains, so they are
  *cross-site*. A `SameSite=Lax` cookie is **not** sent at all, and no CORS configuration changes
  that — `SameSite` is enforced by the browser before the request is made.

Making it work means `SameSite=None; Secure`, which hands back the CSRF surface that `Lax` was
closing for free, which then needs a CSRF token implemented by hand. More code, for a weaker
security posture than the one the project already has. The static host also cannot run the Go
process at all: the worker pool, the in-memory Prometheus registry and `embed.FS` all need a
long-lived process, so the API would have to be hosted elsewhere regardless.

## Consequences

The public role no longer serves `/login` or `/admin` as SPA paths either; they fall through to
the `/:code` wildcard and produce a genuine 404, which is the correct answer for that host.

The admin console is code-split out of the initial JavaScript bundle. **That split is not a
security boundary** — the chunk is still served from `/assets` and can be fetched directly. The
boundary is the server returning 404. The split exists so the public page does not download code
it cannot use.

The SPA learns its role from a placeholder the server substitutes into the HTML shell once at
boot, not from an API call. A request on the landing page to ask "am I an admin instance?" would
cost a round trip on the page that most needs to be fast.
